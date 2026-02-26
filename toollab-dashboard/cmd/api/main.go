package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"toollab-dashboard/internal/executor"
	"toollab-dashboard/pkg/config"
	"toollab-dashboard/pkg/database"
	httpPkg "toollab-dashboard/pkg/http"
)

func main() {
	cfg := config.Load()

	fmt.Println("toollab-dashboard starting...")
	fmt.Printf("  port:     %s\n", cfg.Port)
	fmt.Printf("  db:       %s\n", cfg.DBPath)
	fmt.Printf("  core_dir: %s\n", cfg.CoreDir)

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	migrationsDir := findMigrations()
	if err := database.Migrate(db, migrationsDir); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	exec := executor.New(cfg.CoreDir)
	router := httpPkg.NewRouter(db, exec)

	fmt.Printf("  listening on :%s\n", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func findMigrations() string {
	candidates := []string{
		"migrations",
		"toollab-dashboard/migrations",
		"/app/migrations",
	}
	exe, _ := os.Executable()
	if exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "migrations"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return "migrations"
}
