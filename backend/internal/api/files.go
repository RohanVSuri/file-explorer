package api

import (
	"net/http"
	"net/url"
	"strconv"

	"rohansuri.com/file-explorer/internal/db"
)

// POST /files
// Accepts multipart/form-data with fields: file, parent_id (optional)
func (h *Handlers) uploadFile(w http.ResponseWriter, r *http.Request) {
	// 32MB in memory, overflow to disk — fine for typical uploads.
	// Large files stream through the blob store regardless.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "file field is required")
		return
	}
	defer file.Close()

	var parentID *int64
	if raw := r.FormValue("parent_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_input", "parent_id must be an integer")
			return
		}
		parentID = &id
	}

	hash, size, err := h.store.Upload(r.Context(), file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "upload_failed", "failed to store file")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	node, err := h.db.InsertNode(r.Context(), db.InsertNodeParams{
		ParentID:    parentID,
		Name:        header.Filename,
		Type:        "file",
		Size:        &size,
		MimeType:    &mimeType,
		ContentHash: &hash,
	})
	if err != nil {
		handleDBError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

// GET /files/{id}/content
func (h *Handlers) downloadFile(w http.ResponseWriter, r *http.Request) {
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
	if node.Type != "file" || node.ContentHash == nil {
		writeError(w, http.StatusBadRequest, "invalid_input", "node is not a file")
		return
	}

	f, err := h.store.Open(*node.ContentHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not open file")
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(node.Name))
	// ServeContent handles Range requests, ETags, conditional GET, and Content-Length.
	http.ServeContent(w, r, node.Name, node.UpdatedAt, f)
}
