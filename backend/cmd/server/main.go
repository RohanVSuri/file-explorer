package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"rohansuri.com/file-explorer/internal/api"
)

type config struct {
	ServerPort string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config{
		ServerPort: getEnv("PORT", "8080"),
	}

	log.Printf("Server running on http://localhost:%s", cfg.ServerPort)
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      api.NewRouter(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}