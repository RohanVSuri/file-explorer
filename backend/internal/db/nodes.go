package db

import (
	"context"
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
	return scanNode(row.Scan)
}

func (d *DB) RenameNode(ctx context.Context, id int64, name string) (Node, error) {
	row := d.pool.QueryRow(ctx,
		"UPDATE nodes SET name = $1, updated_at = NOW() WHERE id = $2 RETURNING "+nodeColumns,
		name, id)
	return scanNode(row.Scan)
}

// MoveNode updates parent_id; newParentID nil moves the node to root.
func (d *DB) MoveNode(ctx context.Context, id int64, newParentID *int64) (Node, error) {
	row := d.pool.QueryRow(ctx,
		"UPDATE nodes SET parent_id = $1, updated_at = NOW() WHERE id = $2 RETURNING "+nodeColumns,
		newParentID, id)
	return scanNode(row.Scan)
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

// GetDescendantIDs returns the IDs of a node and all its descendants.
// Used to detect move cycles before committing a move.
func (d *DB) GetDescendantIDs(ctx context.Context, id int64) ([]int64, error) {
	rows, err := d.pool.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT id FROM nodes WHERE id = $1
			UNION ALL
			SELECT n.id FROM nodes n
			JOIN subtree s ON n.parent_id = s.id
		)
		SELECT id FROM subtree
	`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (d *DB) ListTrash(ctx context.Context) ([]Node, error) {
	rows, err := d.pool.Query(ctx,
		"SELECT "+nodeColumns+" FROM nodes WHERE deleted_at IS NOT NULL ORDER BY deleted_at DESC")
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

func (d *DB) HardDeleteNode(ctx context.Context, id int64) error {
	_, err := d.pool.Exec(ctx, "DELETE FROM nodes WHERE id = $1", id)
	return err
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

	// score is an extra column not in Node — scan it separately
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
