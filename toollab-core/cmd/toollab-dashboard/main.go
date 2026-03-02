package main

import (
	"embed"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	artifactRepo "toollab-core/internal/artifact/repository"
	artifactUC "toollab-core/internal/artifact"
	runHandler "toollab-core/internal/run/handler"
	runRepo "toollab-core/internal/run/repository"
	runUC "toollab-core/internal/run"
	"toollab-core/internal/shared"
	targetHandler "toollab-core/internal/target/handler"
	targetRepo "toollab-core/internal/target/repository"
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
	if vertex.Available() {
		llmRunner = llm.NewRunner(vertex, aSvc)
		log.Printf("LLM provider: %s", vertex.Name())
	} else {
		log.Printf("LLM provider: disabled (set GOOGLE_PROJECT_ID + GOOGLE_ACCESS_TOKEN for Vertex)")
	}

	orch := pipeline.NewOrchestrator(tRepo, rRepo, aSvc, steps, llmRunner)
	azH := pipeline.NewHandler(orch)

	tH := targetHandler.New(tSvc)
	rH := runHandler.New(rSvc, aSvc)
	pgH := playground.NewHandler(aSvc)

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
			r.Get("/{target_id}/latest-run", rH.LatestRunForTarget)
		})
		r.Route("/runs", func(r chi.Router) {
			r.Mount("/", rH.RunRoutes())
			r.Route("/{run_id}/playground", func(r chi.Router) {
				r.Mount("/", pgH.Routes())
			})
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
