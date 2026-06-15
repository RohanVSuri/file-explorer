package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"rohansuri.com/file-explorer/internal/db"
	"rohansuri.com/file-explorer/internal/store"
	"net/http"
)

func NewRouter(database *db.DB, blobStore *store.Store) http.Handler {
	h := &Handlers{db: database, store: blobStore}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"http://localhost:5173"},
		AllowedMethods: []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type"},
	}))

	r.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "Hello from the filesystem API"})
	})

	h.routes(r)

	return r
}
