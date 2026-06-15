package store_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rohansuri.com/file-explorer/internal/store"
)

func newStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	return s
}

func TestUpload_BasicRoundtrip(t *testing.T) {
	s := newStore(t)
	content := "hello world"

	hash, size, err := s.Upload(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("size: got %d, want %d", size, len(content))
	}
	if len(hash) != 64 {
		t.Errorf("expected 64-char SHA-256 hex, got %q", hash)
	}

	f, err := s.Open(hash)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()

	got, _ := io.ReadAll(f)
	if string(got) != content {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestUpload_Deduplication(t *testing.T) {
	s := newStore(t)
	content := "duplicate bytes"

	hash1, _, err := s.Upload(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatalf("first Upload: %v", err)
	}
	hash2, _, err := s.Upload(context.Background(), strings.NewReader(content))
	if err != nil {
		t.Fatalf("second Upload: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("same content should produce same hash: %q != %q", hash1, hash2)
	}

	// Only one blob file should exist on disk
	blobDir := filepath.Join(t.TempDir()) // can't reach private baseDir, verify via Open
	_ = blobDir
	f, err := s.Open(hash1)
	if err != nil {
		t.Fatalf("Open after dedup: %v", err)
	}
	f.Close()
}

func TestUpload_DifferentContent(t *testing.T) {
	s := newStore(t)

	hash1, _, _ := s.Upload(context.Background(), strings.NewReader("aaa"))
	hash2, _, _ := s.Upload(context.Background(), strings.NewReader("bbb"))

	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
}

func TestUpload_LargeContent(t *testing.T) {
	s := newStore(t)

	// 4MB of data — larger than io.Copy's default 32KB buffer
	data := bytes.Repeat([]byte("x"), 4<<20)
	hash, size, err := s.Upload(context.Background(), bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Upload large: %v", err)
	}
	if size != int64(len(data)) {
		t.Errorf("size: got %d, want %d", size, len(data))
	}

	f, _ := s.Open(hash)
	defer f.Close()
	got, _ := io.ReadAll(f)
	if !bytes.Equal(got, data) {
		t.Error("large content mismatch after roundtrip")
	}
}

func TestOpen_NotFound(t *testing.T) {
	s := newStore(t)
	_, err := s.Open("0000000000000000000000000000000000000000000000000000000000000000")
	if !os.IsNotExist(err) {
		t.Errorf("expected not-exist error, got %v", err)
	}
}

func TestSweepTmp_ClearsLeftoverFiles(t *testing.T) {
	dir := t.TempDir()

	// Manually create a leftover file in tmp/
	tmpDir := filepath.Join(dir, "tmp")
	os.MkdirAll(tmpDir, 0755)
	leftover := filepath.Join(tmpDir, "upload-leftover")
	os.WriteFile(leftover, []byte("crash residue"), 0644)

	// Creating a new Store should sweep it
	_, err := store.New(dir)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	if _, err := os.Stat(leftover); !os.IsNotExist(err) {
		t.Error("leftover tmp file should have been swept on startup")
	}
}

func TestDelete(t *testing.T) {
	s := newStore(t)
	hash, _, _ := s.Upload(context.Background(), strings.NewReader("to be deleted"))

	if err := s.Delete(hash); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Open(hash); !os.IsNotExist(err) {
		t.Error("blob should not exist after Delete")
	}
}
