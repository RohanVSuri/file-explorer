package db_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"rohansuri.com/file-explorer/internal/db"
)

func testDB(t *testing.T) *db.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	d, err := db.New(context.Background(), url)
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { d.Truncate(context.Background()) })
	return d
}

func ptr[T any](v T) *T { return &v }

// --- InsertNode / GetNode ---

func TestInsertAndGetNode(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	node, err := d.InsertNode(ctx, db.InsertNodeParams{Name: "docs", Type: "folder"})
	if err != nil {
		t.Fatalf("InsertNode: %v", err)
	}
	if node.ID == 0 || node.Name != "docs" || node.Type != "folder" {
		t.Errorf("unexpected node: %+v", node)
	}

	got, err := d.GetNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.ID != node.ID || got.Name != node.Name {
		t.Errorf("GetNode mismatch: got %+v, want %+v", got, node)
	}
}

func TestGetNode_NotFound(t *testing.T) {
	d := testDB(t)
	_, err := d.GetNode(context.Background(), 999999)
	if !errors.Is(err, db.ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

// --- ListChildren ---

func TestListChildren_Root(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	d.InsertNode(ctx, db.InsertNodeParams{Name: "a", Type: "folder"})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "b", Type: "folder"})

	nodes, err := d.ListChildren(ctx, nil, 100, 0)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("want 2 root nodes, got %d", len(nodes))
	}
}

func TestListChildren_Nested(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	parent, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "parent", Type: "folder"})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "child1", Type: "folder", ParentID: &parent.ID})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "child2", Type: "file", ParentID: &parent.ID})

	children, err := d.ListChildren(ctx, &parent.ID, 100, 0)
	if err != nil {
		t.Fatalf("ListChildren: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("want 2 children, got %d", len(children))
	}
	// folders come first
	if children[0].Type != "folder" {
		t.Errorf("want folder first, got %q", children[0].Type)
	}
}

func TestListChildren_ExcludesDeleted(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	parent, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "parent", Type: "folder"})
	child, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "child", Type: "folder", ParentID: &parent.ID})
	d.SoftDeleteSubtree(ctx, child.ID)

	children, _ := d.ListChildren(ctx, &parent.ID, 100, 0)
	if len(children) != 0 {
		t.Errorf("want 0 children after delete, got %d", len(children))
	}
}

func TestListChildren_Pagination(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	for i := range 5 {
		d.InsertNode(ctx, db.InsertNodeParams{Name: string(rune('a'+i)), Type: "folder"})
	}

	page1, _ := d.ListChildren(ctx, nil, 3, 0)
	page2, _ := d.ListChildren(ctx, nil, 3, 3)
	if len(page1) != 3 {
		t.Errorf("page1: want 3, got %d", len(page1))
	}
	if len(page2) != 2 {
		t.Errorf("page2: want 2, got %d", len(page2))
	}
}

// --- Name uniqueness ---

func TestInsertNode_NameConflict(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	d.InsertNode(ctx, db.InsertNodeParams{Name: "dup", Type: "folder"})
	_, err := d.InsertNode(ctx, db.InsertNodeParams{Name: "dup", Type: "folder"})
	if !errors.Is(err, db.ErrNameConflict) {
		t.Errorf("want ErrNameConflict, got %v", err)
	}
}

func TestInsertNode_SameNameDifferentParent(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	a, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "a", Type: "folder"})
	b, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "b", Type: "folder"})

	// Same name "readme" under two different parents — must not conflict
	_, err1 := d.InsertNode(ctx, db.InsertNodeParams{Name: "readme", Type: "file", ParentID: &a.ID})
	_, err2 := d.InsertNode(ctx, db.InsertNodeParams{Name: "readme", Type: "file", ParentID: &b.ID})
	if err1 != nil || err2 != nil {
		t.Errorf("same name in different parents should be allowed: %v, %v", err1, err2)
	}
}

func TestInsertNode_SameNameAfterDelete(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	node, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "reuse-me", Type: "folder"})
	d.SoftDeleteSubtree(ctx, node.ID)

	// Should be allowed to create a new node with the same name now
	_, err := d.InsertNode(ctx, db.InsertNodeParams{Name: "reuse-me", Type: "folder"})
	if err != nil {
		t.Errorf("should allow same name after soft delete, got: %v", err)
	}
}

// --- RenameNode ---

func TestRenameNode(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	node, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "before", Type: "folder"})
	updated, err := d.RenameNode(ctx, node.ID, "after")
	if err != nil {
		t.Fatalf("RenameNode: %v", err)
	}
	if updated.Name != "after" {
		t.Errorf("want 'after', got %q", updated.Name)
	}
}

func TestRenameNode_Conflict(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	d.InsertNode(ctx, db.InsertNodeParams{Name: "existing", Type: "folder"})
	node, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "other", Type: "folder"})

	_, err := d.RenameNode(ctx, node.ID, "existing")
	if !errors.Is(err, db.ErrNameConflict) {
		t.Errorf("want ErrNameConflict, got %v", err)
	}
}

// --- SafeMoveNode ---

func TestSafeMoveNode_Basic(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	src, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "src", Type: "folder"})
	dst, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "dst", Type: "folder"})

	moved, err := d.SafeMoveNode(ctx, src.ID, &dst.ID)
	if err != nil {
		t.Fatalf("SafeMoveNode: %v", err)
	}
	if moved.ParentID == nil || *moved.ParentID != dst.ID {
		t.Errorf("expected parent_id=%d, got %v", dst.ID, moved.ParentID)
	}
}

func TestSafeMoveNode_ToRoot(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	parent, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "parent", Type: "folder"})
	child, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "child", Type: "folder", ParentID: &parent.ID})

	moved, err := d.SafeMoveNode(ctx, child.ID, nil)
	if err != nil {
		t.Fatalf("SafeMoveNode to root: %v", err)
	}
	if moved.ParentID != nil {
		t.Errorf("expected parent_id=nil (root), got %v", moved.ParentID)
	}
}

func TestSafeMoveNode_DirectCycle(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	// A → B; moving A under B creates a cycle
	a, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "A", Type: "folder"})
	b, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "B", Type: "folder", ParentID: &a.ID})

	_, err := d.SafeMoveNode(ctx, a.ID, &b.ID)
	if !errors.Is(err, db.ErrCycleDetected) {
		t.Errorf("want ErrCycleDetected, got %v", err)
	}
}

func TestSafeMoveNode_DeepCycle(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	// root → A → B → C → D; moving A under D creates a cycle
	a, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "A", Type: "folder"})
	b, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "B", Type: "folder", ParentID: &a.ID})
	c, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "C", Type: "folder", ParentID: &b.ID})
	dd, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "D", Type: "folder", ParentID: &c.ID})

	_, err := d.SafeMoveNode(ctx, a.ID, &dd.ID)
	if !errors.Is(err, db.ErrCycleDetected) {
		t.Errorf("want ErrCycleDetected for deep cycle, got %v", err)
	}
}

func TestSafeMoveNode_SelfMove(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	a, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "A", Type: "folder"})
	_, err := d.SafeMoveNode(ctx, a.ID, &a.ID)
	if !errors.Is(err, db.ErrCycleDetected) {
		t.Errorf("moving a node into itself: want ErrCycleDetected, got %v", err)
	}
}

// TestSafeMoveNode_ConcurrentRace proves SELECT FOR UPDATE prevents cycles
// under concurrent moves. Run with: go test -race ./internal/db/...
func TestSafeMoveNode_ConcurrentRace(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	// Build root → A → B → C
	a, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "A", Type: "folder"})
	b, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "B", Type: "folder", ParentID: &a.ID})
	c, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "C", Type: "folder", ParentID: &b.ID})

	// 30 goroutines: half try A→under C, half try C→under A.
	// One direction must win; neither should create a cycle.
	var wg sync.WaitGroup
	for i := range 30 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				d.SafeMoveNode(ctx, a.ID, &c.ID)
			} else {
				d.SafeMoveNode(ctx, c.ID, &a.ID)
			}
		}(i)
	}
	wg.Wait()

	// Verify tree integrity: list root children — a cycle would cause the
	// recursive CTE inside ListChildren to hang or error.
	_, err := d.ListChildren(ctx, nil, 100, 0)
	if err != nil {
		t.Fatalf("ListChildren after concurrent moves failed — possible cycle: %v", err)
	}

	// All three nodes must still be fetchable (none corrupted)
	for _, id := range []int64{a.ID, b.ID, c.ID} {
		if _, err := d.GetNode(ctx, id); err != nil {
			t.Errorf("node %d not fetchable after concurrent moves: %v", id, err)
		}
	}
}

// --- SoftDeleteSubtree ---

func TestSoftDeleteSubtree_Single(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	node, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "gone", Type: "folder"})
	if err := d.SoftDeleteSubtree(ctx, node.ID); err != nil {
		t.Fatalf("SoftDeleteSubtree: %v", err)
	}

	got, _ := d.GetNode(ctx, node.ID)
	if got.DeletedAt == nil {
		t.Error("expected deleted_at to be set")
	}
}

func TestSoftDeleteSubtree_CascadesToChildren(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	parent, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "parent", Type: "folder"})
	child, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "child", Type: "folder", ParentID: &parent.ID})
	grandchild, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "grandchild", Type: "folder", ParentID: &child.ID})

	d.SoftDeleteSubtree(ctx, parent.ID)

	for _, id := range []int64{parent.ID, child.ID, grandchild.ID} {
		n, _ := d.GetNode(ctx, id)
		if n.DeletedAt == nil {
			t.Errorf("node %d: expected deleted_at to be set", id)
		}
	}
}

// --- ListTrash ---

func TestListTrash_TopLevelOnly(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	parent, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "parent", Type: "folder"})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "child", Type: "folder", ParentID: &parent.ID})

	// Deleting parent also marks child — trash should show only parent
	d.SoftDeleteSubtree(ctx, parent.ID)

	trash, err := d.ListTrash(ctx)
	if err != nil {
		t.Fatalf("ListTrash: %v", err)
	}
	if len(trash) != 1 {
		t.Errorf("want 1 item in trash, got %d", len(trash))
	}
	if trash[0].ID != parent.ID {
		t.Errorf("want parent in trash, got id=%d", trash[0].ID)
	}
}

// --- RestoreNode ---

func TestRestoreNode(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	node, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "restore-me", Type: "folder"})
	d.SoftDeleteSubtree(ctx, node.ID)

	restored, err := d.RestoreNode(ctx, node.ID)
	if err != nil {
		t.Fatalf("RestoreNode: %v", err)
	}
	if restored.DeletedAt != nil {
		t.Error("expected deleted_at to be nil after restore")
	}

	// Should appear in listing again
	children, _ := d.ListChildren(ctx, nil, 100, 0)
	var found bool
	for _, n := range children {
		if n.ID == node.ID {
			found = true
		}
	}
	if !found {
		t.Error("restored node should appear in listing")
	}
}

// --- SearchNodes ---

func TestSearchNodes_Global(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	d.InsertNode(ctx, db.InsertNodeParams{Name: "quarterly-report", Type: "file"})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "annual-summary", Type: "file"})

	results, err := d.SearchNodes(ctx, "quarterly", nil)
	if err != nil {
		t.Fatalf("SearchNodes: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'quarterly'")
	}
	if results[0].Name != "quarterly-report" {
		t.Errorf("expected top result 'quarterly-report', got %q", results[0].Name)
	}
}

func TestSearchNodes_Scoped(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	folderA, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "folderA", Type: "folder"})
	folderB, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "folderB", Type: "folder"})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "target", Type: "file", ParentID: &folderA.ID})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "target", Type: "file", ParentID: &folderB.ID})

	// Scoped to folderA — should only return the one inside folderA
	results, err := d.SearchNodes(ctx, "target", &folderA.ID)
	if err != nil {
		t.Fatalf("SearchNodes scoped: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 scoped result, got %d", len(results))
	}
	if results[0].ParentID == nil || *results[0].ParentID != folderA.ID {
		t.Errorf("result should be inside folderA")
	}
}

// --- BlobRefCount ---

func TestBlobRefCount(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	hash := "abc123hash"
	d.InsertNode(ctx, db.InsertNodeParams{Name: "file1.txt", Type: "file", ContentHash: &hash})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "file2.txt", Type: "file", ContentHash: &hash})

	count, err := d.BlobRefCount(ctx, hash)
	if err != nil {
		t.Fatalf("BlobRefCount: %v", err)
	}
	if count != 2 {
		t.Errorf("want refcount 2, got %d", count)
	}
}

func TestBlobRefCount_DropsToZeroAfterDelete(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	hash := "uniquehash123"
	node, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "only.txt", Type: "file", ContentHash: &hash})
	d.HardDeleteSubtree(ctx, node.ID)

	count, _ := d.BlobRefCount(ctx, hash)
	if count != 0 {
		t.Errorf("want refcount 0 after hard delete, got %d", count)
	}
}

// --- HardDeleteSubtree ---

func TestHardDeleteSubtree_SingleFile(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	node, err := d.InsertNode(ctx, db.InsertNodeParams{Name: "lone.txt", Type: "file", ContentHash: ptr("hashA")})
	if err != nil {
		t.Fatalf("InsertNode: %v", err)
	}

	if _, err := d.HardDeleteSubtree(ctx, node.ID); err != nil {
		t.Fatalf("HardDeleteSubtree: %v", err)
	}

	if _, err := d.GetNode(ctx, node.ID); !errors.Is(err, db.ErrNotFound) {
		t.Errorf("want ErrNotFound after hard delete, got %v", err)
	}
}

// Regression: deleting a folder that has descendant files must not fail with a
// FK violation. This is the exact scenario that returned 500 before the fix.
func TestHardDeleteSubtree_FolderWithNestedFiles(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	root, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "root", Type: "folder"})
	sub, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "sub", Type: "folder", ParentID: &root.ID})
	file, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "file.txt", Type: "file", ParentID: &sub.ID, ContentHash: ptr("hashZ")})

	// Soft-delete the subtree first (mirrors what the trash flow does).
	if err := d.SoftDeleteSubtree(ctx, root.ID); err != nil {
		t.Fatalf("SoftDeleteSubtree: %v", err)
	}

	hashes, err := d.HardDeleteSubtree(ctx, root.ID)
	if err != nil {
		t.Fatalf("HardDeleteSubtree: %v", err)
	}

	// All three nodes must be gone.
	for _, id := range []int64{root.ID, sub.ID, file.ID} {
		if _, err := d.GetNode(ctx, id); !errors.Is(err, db.ErrNotFound) {
			t.Errorf("node %d: want ErrNotFound, got %v", id, err)
		}
	}

	// The file's hash must be in the returned slice.
	found := false
	for _, h := range hashes {
		if h == "hashZ" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected hash %q in returned hashes %v", "hashZ", hashes)
	}
}

func TestHardDeleteSubtree_ReturnsHashes(t *testing.T) {
	d := testDB(t)
	ctx := context.Background()

	folder, _ := d.InsertNode(ctx, db.InsertNodeParams{Name: "folder", Type: "folder"})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "a.txt", Type: "file", ParentID: &folder.ID, ContentHash: ptr("hash1")})
	d.InsertNode(ctx, db.InsertNodeParams{Name: "b.txt", Type: "file", ParentID: &folder.ID, ContentHash: ptr("hash2")})

	hashes, err := d.HardDeleteSubtree(ctx, folder.ID)
	if err != nil {
		t.Fatalf("HardDeleteSubtree: %v", err)
	}

	got := make(map[string]bool)
	for _, h := range hashes {
		got[h] = true
	}
	for _, want := range []string{"hash1", "hash2"} {
		if !got[want] {
			t.Errorf("hash %q missing from returned hashes %v", want, hashes)
		}
	}
}
