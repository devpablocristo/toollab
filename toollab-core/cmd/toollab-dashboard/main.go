package main

import (
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	artifactRepo "toollab-core/internal/artifact/repository"
	artifactUC "toollab-core/internal/artifact/usecases"
	discoveryUC "toollab-core/internal/discovery/usecases"
	evidenceUC "toollab-core/internal/evidence/usecases"
	interpretUC "toollab-core/internal/interpretation/usecases"
	runHandler "toollab-core/internal/run/handler"
	runRepo "toollab-core/internal/run/repository"
	runUC "toollab-core/internal/run/usecases"
	runnerUC "toollab-core/internal/runner/usecases"
	"toollab-core/internal/shared"
	analyzeUC "toollab-core/internal/analyze"
	targetHandler "toollab-core/internal/target/handler"
	targetRepo "toollab-core/internal/target/repository"
	targetUC "toollab-core/internal/target/usecases"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	dbPath := env("TOOLLAB_DB_PATH", "./data/toollab.db")
	dataDir := env("TOOLLAB_DATA_DIR", "./data")
	addr := env("TOOLLAB_ADDR", ":8090")

	db, err := shared.OpenDB(dbPath)
	if err != nil {
		log.Fatalf("opening database: %v", err)
	}
	defer db.Close()

	migSQL, err := migrationsFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		log.Fatalf("reading migration: %v", err)
	}
	if err := shared.Migrate(db, []string{string(migSQL)}); err != nil {
		log.Fatalf("running migrations: %v", err)
	}

	tRepo := targetRepo.NewSQLite(db)
	rRepo := runRepo.NewSQLite(db)
	aIdxRepo := artifactRepo.NewSQLite(db)
	aStorage := artifactRepo.NewFSStorage(dataDir)

	tSvc := targetUC.NewService(tRepo)
	rSvc := runUC.NewService(rRepo, tRepo)
	aSvc := artifactUC.NewService(aIdxRepo, aStorage, rRepo)

	runner := runnerUC.NewHTTPRunner()
	artPutter := evidenceUC.NewArtifactPutter(aSvc)
	ingestor := evidenceUC.NewFSIngestor(aStorage, artPutter)

	analyzer := discoveryUC.NewGoAnalyzer()
	dSvc := discoveryUC.NewService(analyzer, aSvc, tRepo)

	dossierBuilder := interpretUC.NewDossierBuilder(aSvc)
	var llmProvider interpretUC.Provider
	vertexProvider := interpretUC.NewVertexProvider()
	if vertexProvider.Available() {
		llmProvider = vertexProvider
		log.Printf("LLM provider: %s", vertexProvider.Name())
	} else {
		llmProvider = interpretUC.NewMockProvider()
		log.Printf("LLM provider: mock (set GOOGLE_PROJECT_ID + GOOGLE_ACCESS_TOKEN for Vertex)")
	}
	interpSvc := interpretUC.NewService(dossierBuilder, llmProvider, aSvc)

	orchestrator := analyzeUC.NewOrchestrator(tRepo, rRepo, aSvc, dSvc, runner, ingestor, interpSvc)

	tH := targetHandler.New(tSvc)
	rH := runHandler.New(rSvc, aSvc)
	azH := analyzeUC.NewHandler(orchestrator)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		shared.WriteJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/targets", func(r chi.Router) {
			r.Mount("/", tH.Routes())
			r.Route("/{target_id}/analyze", func(r chi.Router) {
				r.Mount("/", azH.Routes())
			})
		})
		r.Route("/runs", func(r chi.Router) {
			r.Mount("/", rH.RunRoutes())
		})
	})

	log.Printf("toollab-core listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
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
