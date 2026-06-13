package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"rohansuri.com/file-explorer/internal/api"
	"rohansuri.com/file-explorer/internal/db"
)

type config struct {
	ServerPort  string
	DatabaseURL string
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg := config{
		ServerPort:  getEnv("PORT", "8080"),
		DatabaseURL: getEnv("DATABASE_URL", "postgres://user:password@localhost:5432/file-explorer"),
	}

	ctx := context.Background()

	database, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	log.Println("database connected and migrations applied")

	log.Printf("server running on http://localhost:%s", cfg.ServerPort)
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      api.NewRouter(database),
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
