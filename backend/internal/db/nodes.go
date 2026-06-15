package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type Node struct {
	ID          int64      `json:"id"`
	ParentID    *int64     `json:"parent_id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Size        *int64     `json:"size,omitempty"`
	MimeType    *string    `json:"mime_type,omitempty"`
	ContentHash *string    `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
}

const nodeColumns = `id, parent_id, name, type, size, mime_type, content_hash,
	created_at, updated_at, deleted_at`

func scanNode(scan func(dest ...any) error) (Node, error) {
	var n Node
	err := scan(&n.ID, &n.ParentID, &n.Name, &n.Type,
		&n.Size, &n.MimeType, &n.ContentHash,
		&n.CreatedAt, &n.UpdatedAt, &n.DeletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Node{}, ErrNotFound
	}
	return n, err
}

func (d *DB) GetNode(ctx context.Context, id int64) (Node, error) {
	row := d.pool.QueryRow(ctx,
		"SELECT "+nodeColumns+" FROM nodes WHERE id = $1", id)
	return scanNode(row.Scan)
}

// ListChildren returns direct children of parentID (nil = root), sorted folders-first then alpha.
func (d *DB) ListChildren(ctx context.Context, parentID *int64, limit, offset int) ([]Node, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if parentID == nil {
		rows, err = d.pool.Query(ctx,
			"SELECT "+nodeColumns+" FROM nodes WHERE parent_id IS NULL AND deleted_at IS NULL ORDER BY type DESC, name ASC LIMIT $1 OFFSET $2",
			limit, offset)
	} else {
		rows, err = d.pool.Query(ctx,
			"SELECT "+nodeColumns+" FROM nodes WHERE parent_id = $1 AND deleted_at IS NULL ORDER BY type DESC, name ASC LIMIT $2 OFFSET $3",
			*parentID, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectNodes(rows)
}

type InsertNodeParams struct {
	ParentID    *int64
	Name        string
	Type        string
	Size        *int64
	MimeType    *string
	ContentHash *string
}

func (d *DB) InsertNode(ctx context.Context, p InsertNodeParams) (Node, error) {
	row := d.pool.QueryRow(ctx,
		"INSERT INTO nodes (parent_id, name, type, size, mime_type, content_hash) VALUES ($1,$2,$3,$4,$5,$6) RETURNING "+nodeColumns,
		p.ParentID, p.Name, p.Type, p.Size, p.MimeType, p.ContentHash)
	n, err := scanNode(row.Scan)
	if isUniqueViolation(err) {
		return Node{}, ErrNameConflict
	}
	return n, err
}

func (d *DB) RenameNode(ctx context.Context, id int64, name string) (Node, error) {
	row := d.pool.QueryRow(ctx,
		"UPDATE nodes SET name = $1, updated_at = NOW() WHERE id = $2 RETURNING "+nodeColumns,
		name, id)
	n, err := scanNode(row.Scan)
	if isUniqueViolation(err) {
		return Node{}, ErrNameConflict
	}
	return n, err
}

// SafeMoveNode moves a node to a new parent inside a transaction with cycle detection.
// newParentID nil moves the node to root.
func (d *DB) SafeMoveNode(ctx context.Context, id int64, newParentID *int64) (Node, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return Node{}, err
	}
	defer tx.Rollback(ctx)

	// Lock the node to prevent concurrent moves racing through the cycle check.
	if _, err := tx.Exec(ctx, "SELECT id FROM nodes WHERE id = $1 FOR UPDATE", id); err != nil {
		return Node{}, err
	}

	if newParentID != nil {
		// Walk descendants of the node being moved; target must not be among them.
		rows, err := tx.Query(ctx, `
			WITH RECURSIVE subtree AS (
				SELECT id FROM nodes WHERE id = $1
				UNION ALL
				SELECT n.id FROM nodes n JOIN subtree s ON n.parent_id = s.id
			)
			SELECT id FROM subtree
		`, id)
		if err != nil {
			return Node{}, err
		}
		for rows.Next() {
			var did int64
			if err := rows.Scan(&did); err != nil {
				rows.Close()
				return Node{}, err
			}
			if did == *newParentID {
				rows.Close()
				return Node{}, ErrCycleDetected
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return Node{}, err
		}
	}

	row := tx.QueryRow(ctx,
		"UPDATE nodes SET parent_id = $1, updated_at = NOW() WHERE id = $2 RETURNING "+nodeColumns,
		newParentID, id)
	node, err := scanNode(row.Scan)
	if err != nil {
		return Node{}, err
	}

	return node, tx.Commit(ctx)
}

// SoftDeleteSubtree sets deleted_at on a node and all its descendants.
func (d *DB) SoftDeleteSubtree(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM nodes WHERE id = $1
			UNION ALL
			SELECT n.id FROM nodes n
			JOIN subtree s ON n.parent_id = s.id
			WHERE n.deleted_at IS NULL
		)
		UPDATE nodes SET deleted_at = NOW() WHERE id IN (SELECT id FROM subtree)
	`, id)
	return err
}

// ListTrash returns top-level trashed nodes (excludes children of already-trashed folders).
func (d *DB) ListTrash(ctx context.Context) ([]Node, error) {
	rows, err := d.pool.Query(ctx, `
		SELECT `+nodeColumns+` FROM nodes n
		WHERE n.deleted_at IS NOT NULL
		  AND (n.parent_id IS NULL OR (SELECT deleted_at FROM nodes p WHERE p.id = n.parent_id) IS NULL)
		ORDER BY n.deleted_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectNodes(rows)
}

func (d *DB) RestoreNode(ctx context.Context, id int64) (Node, error) {
	row := d.pool.QueryRow(ctx,
		"UPDATE nodes SET deleted_at = NULL, updated_at = NOW() WHERE id = $1 RETURNING "+nodeColumns, id)
	return scanNode(row.Scan)
}

// HardDeleteSubtree permanently removes a node and all its descendants.
// Returns the content_hashes of deleted file nodes for blob cleanup.
func (d *DB) HardDeleteSubtree(ctx context.Context, id int64) ([]string, error) {
	rows, err := d.pool.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM nodes WHERE id = $1
			UNION ALL
			SELECT n.id FROM nodes n JOIN subtree s ON n.parent_id = s.id
		)
		DELETE FROM nodes WHERE id IN (SELECT id FROM subtree)
		RETURNING content_hash
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var hashes []string
	for rows.Next() {
		var h *string
		if err := rows.Scan(&h); err != nil {
			return nil, err
		}
		if h != nil {
			hashes = append(hashes, *h)
		}
	}
	return hashes, rows.Err()
}

func (d *DB) SearchNodes(ctx context.Context, query string, parentID *int64) ([]Node, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if parentID == nil {
		rows, err = d.pool.Query(ctx, `
			SELECT `+nodeColumns+`, similarity(name, $1) AS score
			FROM nodes
			WHERE name % $1 AND deleted_at IS NULL
			ORDER BY score DESC
			LIMIT 20
		`, query)
	} else {
		rows, err = d.pool.Query(ctx, `
			WITH RECURSIVE subtree AS (
				SELECT id FROM nodes WHERE id = $2
				UNION ALL
				SELECT n.id FROM nodes n JOIN subtree s ON n.parent_id = s.id
			)
			SELECT `+nodeColumns+`, similarity(name, $1) AS score
			FROM nodes
			WHERE id IN (SELECT id FROM subtree)
			  AND name % $1
			  AND deleted_at IS NULL
			ORDER BY score DESC
			LIMIT 20
		`, query, *parentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var score float64
		if err := rows.Scan(&n.ID, &n.ParentID, &n.Name, &n.Type,
			&n.Size, &n.MimeType, &n.ContentHash,
			&n.CreatedAt, &n.UpdatedAt, &n.DeletedAt, &score); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// BlobRefCount returns how many non-deleted nodes reference a given content hash.
func (d *DB) BlobRefCount(ctx context.Context, hash string) (int, error) {
	var count int
	err := d.pool.QueryRow(ctx,
		"SELECT COUNT(*) FROM nodes WHERE content_hash = $1 AND deleted_at IS NULL", hash,
	).Scan(&count)
	return count, err
}

func collectNodes(rows pgx.Rows) ([]Node, error) {
	var nodes []Node
	for rows.Next() {
		n, err := scanNode(rows.Scan)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}
