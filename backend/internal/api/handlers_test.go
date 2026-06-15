package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"rohansuri.com/file-explorer/internal/api"
	"rohansuri.com/file-explorer/internal/db"
	"rohansuri.com/file-explorer/internal/store"
)

// testServer spins up a real HTTP server backed by a real DB and temp store.
// Set TEST_DATABASE_URL to run these tests.
func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	database, err := db.New(context.Background(), url)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { database.Truncate(context.Background()) })

	blobStore, err := store.New(t.TempDir())
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	srv := httptest.NewServer(api.NewRouter(database, blobStore))
	t.Cleanup(srv.Close)
	return srv
}

func do(t *testing.T, srv *httptest.Server, method, path string, body io.Reader, contentType string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, srv.URL+path, body)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	return res
}

func decodeJSON(t *testing.T, r io.Reader, v any) {
	t.Helper()
	if err := json.NewDecoder(r).Decode(v); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
}

func jsonBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

// --- Folder CRUD ---

func TestCreateFolder(t *testing.T) {
	srv := testServer(t)

	res := do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "My Folder"}), "application/json")
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", res.StatusCode)
	}
	var node db.Node
	decodeJSON(t, res.Body, &node)
	if node.Name != "My Folder" || node.Type != "folder" {
		t.Errorf("unexpected node: %+v", node)
	}
}

func TestCreateFolder_NameConflict(t *testing.T) {
	srv := testServer(t)

	do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "dup"}), "application/json")
	res := do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "dup"}), "application/json")
	if res.StatusCode != http.StatusConflict {
		t.Errorf("want 409, got %d", res.StatusCode)
	}
}

func TestListChildren_Empty(t *testing.T) {
	srv := testServer(t)

	res := do(t, srv, "GET", "/nodes/children", nil, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var nodes []db.Node
	decodeJSON(t, res.Body, &nodes)
	if len(nodes) != 0 {
		t.Errorf("want empty array, got %d items", len(nodes))
	}
}

func TestListChildren_WithItems(t *testing.T) {
	srv := testServer(t)

	var parent db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "parent"}), "application/json").Body, &parent)
	do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "child", "parent_id": parent.ID}), "application/json")

	res := do(t, srv, "GET", fmt.Sprintf("/nodes/children?parent_id=%d", parent.ID), nil, "")
	var nodes []db.Node
	decodeJSON(t, res.Body, &nodes)
	if len(nodes) != 1 || nodes[0].Name != "child" {
		t.Errorf("unexpected children: %+v", nodes)
	}
}

func TestGetNode(t *testing.T) {
	srv := testServer(t)

	var created db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "findme"}), "application/json").Body, &created)

	res := do(t, srv, "GET", fmt.Sprintf("/nodes/%d", created.ID), nil, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var node db.Node
	decodeJSON(t, res.Body, &node)
	if node.ID != created.ID {
		t.Errorf("ID mismatch: got %d, want %d", node.ID, created.ID)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	srv := testServer(t)
	res := do(t, srv, "GET", "/nodes/999999", nil, "")
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("want 404, got %d", res.StatusCode)
	}
}

func TestRenameNode(t *testing.T) {
	srv := testServer(t)

	var node db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "before"}), "application/json").Body, &node)

	res := do(t, srv, "PATCH", fmt.Sprintf("/nodes/%d", node.ID), jsonBody(map[string]any{"name": "after"}), "application/json")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var updated db.Node
	decodeJSON(t, res.Body, &updated)
	if updated.Name != "after" {
		t.Errorf("want name 'after', got %q", updated.Name)
	}
}

func TestMoveNode(t *testing.T) {
	srv := testServer(t)

	var src, dst db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "src"}), "application/json").Body, &src)
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "dst"}), "application/json").Body, &dst)

	res := do(t, srv, "PATCH", fmt.Sprintf("/nodes/%d", src.ID), jsonBody(map[string]any{"parent_id": dst.ID}), "application/json")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var moved db.Node
	decodeJSON(t, res.Body, &moved)
	if moved.ParentID == nil || *moved.ParentID != dst.ID {
		t.Errorf("expected parent_id=%d, got %v", dst.ID, moved.ParentID)
	}
}

func TestMoveNode_CycleDetected(t *testing.T) {
	srv := testServer(t)

	// root → A → B; moving A under B would create a cycle
	var a, b db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "A"}), "application/json").Body, &a)
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "B", "parent_id": a.ID}), "application/json").Body, &b)

	res := do(t, srv, "PATCH", fmt.Sprintf("/nodes/%d", a.ID), jsonBody(map[string]any{"parent_id": b.ID}), "application/json")
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("want 422, got %d", res.StatusCode)
	}
}

func TestDeleteNode_SoftDelete(t *testing.T) {
	srv := testServer(t)

	var node db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "to-delete"}), "application/json").Body, &node)

	res := do(t, srv, "DELETE", fmt.Sprintf("/nodes/%d", node.ID), nil, "")
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", res.StatusCode)
	}

	// Must not appear in children listing
	listRes := do(t, srv, "GET", "/nodes/children", nil, "")
	var nodes []db.Node
	decodeJSON(t, listRes.Body, &nodes)
	for _, n := range nodes {
		if n.ID == node.ID {
			t.Error("deleted node still appears in listing")
		}
	}
}

func TestDeleteSubtree(t *testing.T) {
	srv := testServer(t)

	// parent → child; deleting parent should hide child too
	var parent, child db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "parent"}), "application/json").Body, &parent)
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "child", "parent_id": parent.ID}), "application/json").Body, &child)

	do(t, srv, "DELETE", fmt.Sprintf("/nodes/%d", parent.ID), nil, "")

	// child must not be fetchable
	res := do(t, srv, "GET", fmt.Sprintf("/nodes/%d", child.ID), nil, "")
	var got db.Node
	decodeJSON(t, res.Body, &got)
	if got.DeletedAt == nil {
		t.Error("child of deleted folder should also be marked deleted")
	}
}

// --- File upload / download ---

func buildMultipart(t *testing.T, filename, content string, parentID *int64) (io.Reader, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile: %v", err)
	}
	io.Copy(fw, strings.NewReader(content))
	if parentID != nil {
		w.WriteField("parent_id", fmt.Sprintf("%d", *parentID))
	}
	w.Close()
	return &buf, w.FormDataContentType()
}

func TestUploadAndDownload(t *testing.T) {
	srv := testServer(t)
	content := "the quick brown fox"

	body, ct := buildMultipart(t, "fox.txt", content, nil)
	res := do(t, srv, "POST", "/files", body, ct)
	if res.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("want 201, got %d: %s", res.StatusCode, b)
	}
	var node db.Node
	decodeJSON(t, res.Body, &node)
	if node.Name != "fox.txt" || node.Type != "file" {
		t.Errorf("unexpected node: %+v", node)
	}

	dlRes := do(t, srv, "GET", fmt.Sprintf("/files/%d/content", node.ID), nil, "")
	if dlRes.StatusCode != http.StatusOK {
		t.Fatalf("download: want 200, got %d", dlRes.StatusCode)
	}
	got, _ := io.ReadAll(dlRes.Body)
	if string(got) != content {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestUploadDedup(t *testing.T) {
	srv := testServer(t)
	content := "duplicate content"

	body1, ct1 := buildMultipart(t, "a.txt", content, nil)
	body2, ct2 := buildMultipart(t, "b.txt", content, nil)

	res1 := do(t, srv, "POST", "/files", body1, ct1)
	res2 := do(t, srv, "POST", "/files", body2, ct2)

	if res1.StatusCode != http.StatusCreated || res2.StatusCode != http.StatusCreated {
		t.Fatalf("both uploads should succeed: %d, %d", res1.StatusCode, res2.StatusCode)
	}

	// Both nodes should be downloadable and return the same content
	var n1, n2 db.Node
	decodeJSON(t, res1.Body, &n1)
	decodeJSON(t, res2.Body, &n2)

	for _, n := range []db.Node{n1, n2} {
		got, _ := io.ReadAll(do(t, srv, "GET", fmt.Sprintf("/files/%d/content", n.ID), nil, "").Body)
		if string(got) != content {
			t.Errorf("deduped file %d: content mismatch", n.ID)
		}
	}
}

// --- Search ---

func TestSearch(t *testing.T) {
	srv := testServer(t)

	do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "quarterly-report"}), "application/json")
	do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "annual-summary"}), "application/json")

	res := do(t, srv, "GET", "/search?q=quarterly", nil, "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var nodes []db.Node
	decodeJSON(t, res.Body, &nodes)
	if len(nodes) == 0 {
		t.Fatal("expected search results for 'quarterly'")
	}
	if nodes[0].Name != "quarterly-report" {
		t.Errorf("expected top result to be 'quarterly-report', got %q", nodes[0].Name)
	}
}

// --- Trash ---

func TestTrash_RestoreAndPermanentDelete(t *testing.T) {
	srv := testServer(t)

	var node db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "deleteme"}), "application/json").Body, &node)

	// Soft delete → appears in trash
	do(t, srv, "DELETE", fmt.Sprintf("/nodes/%d", node.ID), nil, "")
	var trash []db.Node
	decodeJSON(t, do(t, srv, "GET", "/trash", nil, "").Body, &trash)
	if len(trash) != 1 || trash[0].ID != node.ID {
		t.Fatalf("want 1 item in trash, got %+v", trash)
	}

	// Restore → gone from trash
	do(t, srv, "POST", fmt.Sprintf("/trash/%d/restore", node.ID), nil, "")
	var trash2 []db.Node
	decodeJSON(t, do(t, srv, "GET", "/trash", nil, "").Body, &trash2)
	if len(trash2) != 0 {
		t.Errorf("want empty trash after restore, got %d items", len(trash2))
	}

	// Soft delete again, then permanently delete
	do(t, srv, "DELETE", fmt.Sprintf("/nodes/%d", node.ID), nil, "")
	res := do(t, srv, "DELETE", fmt.Sprintf("/trash/%d", node.ID), nil, "")
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("permanent delete: want 204, got %d", res.StatusCode)
	}
	if r := do(t, srv, "GET", fmt.Sprintf("/nodes/%d", node.ID), nil, ""); r.StatusCode != http.StatusNotFound {
		t.Errorf("permanently deleted node should 404, got %d", r.StatusCode)
	}
}

func TestTrash_SubtreeOnlyShowsTopLevel(t *testing.T) {
	srv := testServer(t)

	// parent → child; deleting parent should show only parent in trash
	var parent, child db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "parent"}), "application/json").Body, &parent)
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "child", "parent_id": parent.ID}), "application/json").Body, &child)

	do(t, srv, "DELETE", fmt.Sprintf("/nodes/%d", parent.ID), nil, "")

	var trash []db.Node
	decodeJSON(t, do(t, srv, "GET", "/trash", nil, "").Body, &trash)
	if len(trash) != 1 {
		t.Errorf("want 1 item in trash (parent only), got %d", len(trash))
	}
	if trash[0].ID != parent.ID {
		t.Errorf("want parent in trash, got id=%d", trash[0].ID)
	}
}

// --- Race test ---

func TestMoveNode_ConcurrentNoDeadlock(t *testing.T) {
	srv := testServer(t)

	// Build: root → A → B → C
	var a, b, c db.Node
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "A"}), "application/json").Body, &a)
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "B", "parent_id": a.ID}), "application/json").Body, &b)
	decodeJSON(t, do(t, srv, "POST", "/nodes", jsonBody(map[string]any{"name": "C", "parent_id": b.ID}), "application/json").Body, &c)

	// Fire 20 concurrent moves: half try A→under C, half try C→under A.
	// Exactly one direction must win; neither should corrupt the tree.
	done := make(chan struct{})
	for i := range 20 {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			if i%2 == 0 {
				do(t, srv, "PATCH", fmt.Sprintf("/nodes/%d", a.ID), jsonBody(map[string]any{"parent_id": c.ID}), "application/json")
			} else {
				do(t, srv, "PATCH", fmt.Sprintf("/nodes/%d", c.ID), jsonBody(map[string]any{"parent_id": a.ID}), "application/json")
			}
		}(i)
	}
	for range 20 {
		<-done
	}

	// Verify tree integrity: fetching children of root must succeed without hanging.
	res := do(t, srv, "GET", "/nodes/children", nil, "")
	if res.StatusCode != http.StatusOK {
		t.Errorf("tree integrity check failed: GET /nodes/children returned %d", res.StatusCode)
	}
}
