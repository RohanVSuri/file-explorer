package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"rohansuri.com/file-explorer/internal/db"
)

// GET /nodes/children?parent_id=<id>
// parent_id absent → list root
func (h *Handlers) listChildren(w http.ResponseWriter, r *http.Request) {
	var parentID *int64
	if raw := r.URL.Query().Get("parent_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "parent_id must be an integer")
			return
		}
		parentID = &id
	}

	limit := 100
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	nodes, err := h.db.ListChildren(r.Context(), parentID, limit, offset)
	if err != nil {
		handleDBError(w, err)
		return
	}
	if nodes == nil {
		nodes = []db.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// GET /nodes/{id}
func (h *Handlers) getNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	node, err := h.db.GetNode(r.Context(), id)
	if err != nil {
		handleDBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// POST /nodes
// Body: { "name": "...", "parent_id": <int|null> }
func (h *Handlers) createFolder(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name     string `json:"name"`
		ParentID *int64 `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON")
		return
	}
	if body.Name == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "name is required")
		return
	}

	node, err := h.db.InsertNode(r.Context(), db.InsertNodeParams{
		ParentID: body.ParentID,
		Name:     body.Name,
		Type:     "folder",
	})
	if err != nil {
		handleDBError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

// PATCH /nodes/{id}
// Body: { "name": "..." } and/or { "parent_id": <int|null> }
// Uses raw map decoding so we can distinguish absent vs explicit null for parent_id.
func (h *Handlers) updateNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "invalid JSON")
		return
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_input", "body must contain name or parent_id")
		return
	}

	var node db.Node

	if raw, ok := body["name"]; ok {
		var name string
		if err := json.Unmarshal(raw, &name); err != nil || name == "" {
			writeError(w, http.StatusBadRequest, "invalid_input", "name must be a non-empty string")
			return
		}
		node, err = h.db.RenameNode(r.Context(), id, name)
		if err != nil {
			handleDBError(w, err)
			return
		}
	}

	if raw, ok := body["parent_id"]; ok {
		var parentID *int64
		if string(raw) != "null" {
			var pid int64
			if err := json.Unmarshal(raw, &pid); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_input", "parent_id must be an integer or null")
				return
			}
			parentID = &pid
		}
		node, err = h.db.SafeMoveNode(r.Context(), id, parentID)
		if err != nil {
			handleDBError(w, err)
			return
		}
	}

	writeJSON(w, http.StatusOK, node)
}

// DELETE /nodes/{id}
func (h *Handlers) deleteNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	if err := h.db.SoftDeleteSubtree(r.Context(), id); err != nil {
		handleDBError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /search?q=<query>&parent_id=<id>
func (h *Handlers) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeError(w, http.StatusBadRequest, "invalid_input", "q is required")
		return
	}

	var parentID *int64
	if raw := r.URL.Query().Get("parent_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "parent_id must be an integer")
			return
		}
		parentID = &id
	}

	nodes, err := h.db.SearchNodes(r.Context(), q, parentID)
	if err != nil {
		handleDBError(w, err)
		return
	}
	if nodes == nil {
		nodes = []db.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// GET /trash
func (h *Handlers) listTrash(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.db.ListTrash(r.Context())
	if err != nil {
		handleDBError(w, err)
		return
	}
	if nodes == nil {
		nodes = []db.Node{}
	}
	writeJSON(w, http.StatusOK, nodes)
}

// POST /trash/{id}/restore
func (h *Handlers) restoreNode(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}
	node, err := h.db.RestoreNode(r.Context(), id)
	if err != nil {
		handleDBError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

// DELETE /trash/{id}
func (h *Handlers) permanentDelete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	}

	hashes, err := h.db.HardDeleteSubtree(r.Context(), id)
	if err != nil {
		handleDBError(w, err)
		return
	}

	// Clean up any blobs that are no longer referenced by any remaining node.
	for _, hash := range hashes {
		if count, err := h.db.BlobRefCount(r.Context(), hash); err == nil && count == 0 {
			h.store.Delete(hash)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
