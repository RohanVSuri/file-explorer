CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE TABLE nodes (
  id           BIGSERIAL PRIMARY KEY,
  parent_id    BIGINT REFERENCES nodes(id),
  name         TEXT NOT NULL,
  type         TEXT NOT NULL CHECK (type IN ('file', 'folder')),
  size         BIGINT,
  mime_type    TEXT,
  content_hash TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at   TIMESTAMPTZ
);

-- COALESCE handles root nodes (parent_id IS NULL) so two root-level 'foo' entries are rejected
CREATE UNIQUE INDEX nodes_parent_name_uidx ON nodes(COALESCE(parent_id, 0), name) WHERE deleted_at IS NULL;
CREATE INDEX nodes_name_trgm_idx ON nodes USING gin(name gin_trgm_ops);
