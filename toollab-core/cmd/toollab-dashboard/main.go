package main

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/mattn/go-sqlite3"

	"github.com/devpablocristo/core/http/go/httpjson"

	artifactUC "toollab-core/internal/artifact"
	runUC "toollab-core/internal/run"
	targetUC "toollab-core/internal/target"

	"toollab-core/internal/abuse"
	astdiscovery "toollab-core/internal/astdiscovery"
	"toollab-core/internal/authmatrix"
	"toollab-core/internal/confirm"
	"toollab-core/internal/fuzz"
	"toollab-core/internal/llm"
	"toollab-core/internal/logic"
	"toollab-core/internal/pipeline"
	"toollab-core/internal/playground"
	preflightUC "toollab-core/internal/preflight"
	"toollab-core/internal/repoaudit"
	"toollab-core/internal/report"
	"toollab-core/internal/schema"
	"toollab-core/internal/smoke"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	dbPath := env("TOOLLAB_DB_PATH", "./data/toollab.db")
	dataDir := env("TOOLLAB_DATA_DIR", "./data")
	addr := env("TOOLLAB_ADDR", ":8090")

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	statements, err := migrationStatements()
	if err != nil {
		log.Fatalf("reading migrations: %v", err)
	}
	if err := migrate(db, statements); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	tRepo := targetUC.NewSQLite(db)
	rRepo := runUC.NewSQLite(db)
	aIdxRepo := artifactUC.NewSQLite(db)
	aStorage := artifactUC.NewFSStorage(dataDir)

	tSvc := targetUC.NewService(tRepo)
	rSvc := runUC.NewService(rRepo, tRepo)
	aSvc := artifactUC.NewService(aIdxRepo, aStorage, rRepo)

	steps := []pipeline.StepRunner{
		preflightUC.New(),
		astdiscovery.New(),
		schema.New(),
		smoke.New(),
		authmatrix.New(),
		fuzz.New(),
		logic.New(),
		abuse.New(),
		confirm.New(),
		report.New(aSvc),
	}

	var llmRunner pipeline.LLMRunner
	vertex := llm.NewVertexProvider()
	llmRunner = llm.NewRunner(vertex, aSvc)
	if vertex.Available() {
		log.Printf("LLM provider: %s", vertex.Name())
	} else {
		log.Printf("LLM provider: %s (temporarily unavailable at startup; will retry on each run)", vertex.Name())
	}

	orch := pipeline.NewOrchestrator(tRepo, rRepo, aSvc, steps, llmRunner)
	azH := pipeline.NewHandler(orch)

	tH := targetUC.New(tSvc)
	rH := runUC.New(rSvc, aSvc)
	pgH := playground.NewHandler(aSvc)
	v2Store := repoaudit.NewStore(db)
	v2Engine := repoaudit.NewEngine(v2Store, dataDir)
	v2H := repoaudit.NewHandler(v2Store, v2Engine)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpjson.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/targets", func(r chi.Router) {
			r.Mount("/", tH.Routes())
			r.Route("/{target_id}/analyze", func(r chi.Router) {
				r.Mount("/", azH.Routes())
			})
			r.Get("/{target_id}/latest-run", rH.LatestRunForTarget)
		})
		r.Route("/runs", func(r chi.Router) {
			r.Mount("/", rH.RunRoutes())
			r.Route("/{run_id}/playground", func(r chi.Router) {
				r.Mount("/", pgH.Routes())
			})
		})
	})
	r.Mount("/api/v2", v2H.Routes())

	log.Printf("toollab-core listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}

func migrationStatements() ([]string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	statements := make([]string, 0, len(names))
	for _, name := range names {
		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}
		statements = append(statements, string(data))
	}
	return statements, nil
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
				if isBenignMigrationError(err) {
					continue
				}
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
		}
	}
	return nil
}

func isBenignMigrationError(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column name")
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
