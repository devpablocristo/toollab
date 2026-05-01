package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"

	"toollab-core/internal/projectaudit"
)

func main() {
	dbPath := env("TOOLLAB_DB_PATH", "./data/toollab.db")
	dataDir := env("TOOLLAB_DATA_DIR", "./data")
	addr := env("TOOLLAB_ADDR", ":8090")

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	statements, err := projectaudit.MigrationStatements()
	if err != nil {
		log.Fatalf("reading migrations: %v", err)
	}
	if err := migrate(db, statements); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	store := projectaudit.NewStore(db)
	engine := projectaudit.NewEngine(store, dataDir)
	handler := projectaudit.NewHandler(store, engine)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})
	r.Mount("/api", handler.Routes())

	log.Printf("api listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dsn+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("opening db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("pinging db: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB, statements []string) error {
	for i, stmt := range statements {
		log.Printf("applying migration %d", i+1)
		for _, part := range strings.Split(stmt, ";") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if _, err := db.Exec(part); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
		}
	}
	return nil
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
