package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	web "golangkanban/internal/http"
	"golangkanban/internal/store"
)

const defaultDatabaseURL = "postgres://golangkanban:golangkanban@localhost:5432/golangkanban?sslmode=disable"

func main() {
	addr := getenv("ADDR", ":8080")
	databaseURL := getenv("DATABASE_URL", defaultDatabaseURL)

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping database: %v", err)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           web.New(store.New(db)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("GolangKanban listening on http://localhost%s", addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("listen: %v", err)
	}
}

func getenv(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
