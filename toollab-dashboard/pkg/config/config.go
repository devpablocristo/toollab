package config

import "os"

type Config struct {
	Port       string
	DBPath     string
	OllamaURL  string
	OllamaModel string
	CoreDir    string
}

func Load() Config {
	return Config{
		Port:        envOr("TOOLLAB_DASHBOARD_PORT", "8090"),
		DBPath:      envOr("TOOLLAB_DB_PATH", "data/toollab.db"),
		OllamaURL:   envOr("OLLAMA_URL", "http://localhost:11434"),
		OllamaModel: envOr("OLLAMA_MODEL", "llama3.2"),
		CoreDir:     envOr("TOOLLAB_CORE_DIR", "../toollab-core"),
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
