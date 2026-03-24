// Package llm provides providers and runners for docs/audit generation.
package llm

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"time"

	coreai "github.com/devpablocristo/core/ai/go"
	"golang.org/x/oauth2/google"
)

// VertexProvider calls Vertex AI for LLM report generation.
// Usa core/ai/go.VertexAI internamente.
type VertexProvider struct {
	provider *coreai.VertexAI
	model    string
}

func NewVertexProvider() *VertexProvider {
	projectID := firstNonEmptyEnv("GOOGLE_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT")
	if projectID == "" {
		projectID = projectIDFromADC()
	}
	region := envOr("GOOGLE_REGION", "us-central1")
	model := envOr("GOOGLE_LLM_MODEL", "gemini-2.5-flash")

	tokenSource := func(ctx context.Context) (string, error) {
		// Primero chequear env var directa
		if token := strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN")); token != "" {
			return token, nil
		}
		// Fallback a ADC
		ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return "", err
		}
		tok, err := ts.Token()
		if err != nil {
			return "", err
		}
		return tok.AccessToken, nil
	}

	return &VertexProvider{
		provider: coreai.NewVertexAI(projectID, region, tokenSource,
			coreai.WithVertexModel(model),
			coreai.WithVertexTimeout(8*time.Minute),
		),
		model: model,
	}
}

func (p *VertexProvider) Available() bool {
	projectID := firstNonEmptyEnv("GOOGLE_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT")
	if projectID == "" {
		projectID = projectIDFromADC()
	}
	if strings.TrimSpace(projectID) == "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN")) != "" {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return false
	}
	tok, err := ts.Token()
	return err == nil && tok != nil && strings.TrimSpace(tok.AccessToken) != ""
}

func (p *VertexProvider) Name() string { return "vertex-" + p.model }

// RawPrompt sends a prompt expecting JSON back (constrained decoding).
func (p *VertexProvider) RawPrompt(ctx context.Context, prompt string) ([]byte, error) {
	return coreai.RawPrompt(ctx, p.provider, prompt,
		coreai.WithMaxTokens(65536),
		coreai.WithTemperature(0.2),
		coreai.WithRetries(3, 5*time.Second),
	)
}

// TextPrompt sends a prompt expecting free-form text back (no JSON constraint).
func (p *VertexProvider) TextPrompt(ctx context.Context, prompt string) (string, error) {
	return coreai.TextPrompt(ctx, p.provider, prompt,
		coreai.WithMaxTokens(4096),
		coreai.WithTemperature(0.2),
		coreai.WithRetries(3, 5*time.Second),
	)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func projectIDFromADC() string {
	credPath := strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"))
	if credPath == "" {
		return ""
	}
	data, err := os.ReadFile(credPath)
	if err != nil {
		return ""
	}
	var adc struct {
		QuotaProjectID string `json:"quota_project_id"`
		ProjectID      string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &adc); err != nil {
		return ""
	}
	if strings.TrimSpace(adc.QuotaProjectID) != "" {
		return strings.TrimSpace(adc.QuotaProjectID)
	}
	return strings.TrimSpace(adc.ProjectID)
}
