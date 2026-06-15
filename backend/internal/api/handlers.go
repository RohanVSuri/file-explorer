package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"rohansuri.com/file-explorer/internal/db"
	"rohansuri.com/file-explorer/internal/store"
)

type Handlers struct {
	db    *db.DB
	store *store.Store
}

func (h *Handlers) routes(r chi.Router) {
	r.Get("/nodes/children", h.listChildren)
	r.Get("/nodes/{id}", h.getNode)
	r.Post("/nodes", h.createFolder)
	r.Patch("/nodes/{id}", h.updateNode)
	r.Delete("/nodes/{id}", h.deleteNode)

	r.Post("/files", h.uploadFile)
	r.Get("/files/{id}/content", h.downloadFile)

	r.Get("/search", h.search)

	r.Get("/trash", h.listTrash)
	r.Post("/trash/{id}/restore", h.restoreNode)
	r.Delete("/trash/{id}", h.permanentDelete)
}

// parseID reads a chi URL param and parses it as int64.
func parseID(r *http.Request, param string) (int64, error) {
	s := chi.URLParam(r, param)
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id %q", s)
	}
	return id, nil
}

// handleDBError maps sentinel db errors to appropriate HTTP responses.
func handleDBError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "node not found")
	case errors.Is(err, db.ErrNameConflict):
		writeError(w, http.StatusConflict, "name_conflict", "a file or folder with that name already exists")
	case errors.Is(err, db.ErrCycleDetected):
		writeError(w, http.StatusUnprocessableEntity, "cycle_detected", "move would create a cycle")
	default:
		writeError(w, http.StatusInternalServerError, "internal_error", "an unexpected error occurred")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
