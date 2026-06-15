package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Store struct {
	baseDir string
}

func New(baseDir string) (*Store, error) {
	s := &Store{baseDir: baseDir}
	if err := os.MkdirAll(s.tmpDir(), 0755); err != nil {
		return nil, fmt.Errorf("create tmp dir: %w", err)
	}
	if err := os.MkdirAll(s.blobDir(), 0755); err != nil {
		return nil, fmt.Errorf("create blob dir: %w", err)
	}
	// Sweep any incomplete uploads left by a previous crash.
	if err := s.sweepTmp(); err != nil {
		return nil, fmt.Errorf("sweep tmp: %w", err)
	}
	return s, nil
}

// Upload streams r into the content-addressable store and returns the SHA-256
// hash and byte count. The write is atomic: bytes land in tmp/ first, then
// os.Rename moves them to blobs/ in a single syscall.
func (s *Store) Upload(_ context.Context, r io.Reader) (hash string, size int64, err error) {
	tmp, err := os.CreateTemp(s.tmpDir(), "upload-*")
	if err != nil {
		return "", 0, fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	hasher := sha256.New()
	tee := io.TeeReader(r, hasher)
	if size, err = io.Copy(tmp, tee); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("write upload: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("sync upload: %w", err)
	}
	tmp.Close() // done writing; close before any rename/remove

	hash = hex.EncodeToString(hasher.Sum(nil))
	dest := s.blobPath(hash)

	if err = os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("create blob dir: %w", err)
	}

	if _, statErr := os.Stat(dest); os.IsNotExist(statErr) {
		if err = os.Rename(tmpPath, dest); err != nil {
			os.Remove(tmpPath)
			return "", 0, fmt.Errorf("commit blob: %w", err)
		}
	} else {
		os.Remove(tmpPath)
	}
	return hash, size, nil
}

// Open returns a readable file handle for the blob with the given hash.
func (s *Store) Open(hash string) (*os.File, error) {
	return os.Open(s.blobPath(hash))
}

// Delete removes a blob from disk. Should only be called when no nodes
// reference the hash (refcount = 0).
func (s *Store) Delete(hash string) error {
	return os.Remove(s.blobPath(hash))
}

func (s *Store) blobPath(hash string) string {
	return filepath.Join(s.blobDir(), hash[:2], hash[2:4], hash)
}

func (s *Store) blobDir() string { return filepath.Join(s.baseDir, "blobs") }
func (s *Store) tmpDir() string  { return filepath.Join(s.baseDir, "tmp") }

func (s *Store) sweepTmp() error {
	entries, err := os.ReadDir(s.tmpDir())
	if err != nil {
		return err
	}
	for _, e := range entries {
		os.Remove(filepath.Join(s.tmpDir(), e.Name()))
	}
	return nil
}
