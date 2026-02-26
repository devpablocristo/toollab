package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

// vertexProvider uses Vertex AI (aiplatform.googleapis.com).
// Authenticates via `gcloud auth print-access-token` or GOOGLE_ACCESS_TOKEN env var.
// Requires GOOGLE_PROJECT_ID and optionally GOOGLE_REGION.
type vertexProvider struct {
	projectID string
	region    string
	model     string
	http      *http.Client
}

func newVertexProvider() *vertexProvider {
	region := os.Getenv("GOOGLE_REGION")
	if region == "" {
		region = "us-central1"
	}
	model := os.Getenv("VERTEX_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &vertexProvider{
		projectID: os.Getenv("GOOGLE_PROJECT_ID"),
		region:    region,
		model:     model,
		http:      &http.Client{Timeout: 2 * time.Minute},
	}
}

func (p *vertexProvider) Name() string {
	return fmt.Sprintf("vertex/%s", p.model)
}

func (p *vertexProvider) Available(ctx context.Context) bool {
	if p.projectID == "" {
		return false
	}
	_, err := p.getToken()
	return err == nil
}

func (p *vertexProvider) Interpret(ctx context.Context, fullContext string) (string, error) {
	if p.projectID == "" {
		return "", fmt.Errorf("GOOGLE_PROJECT_ID not set")
	}

	token, err := p.getToken()
	if err != nil {
		return "", fmt.Errorf("vertex auth: %w", err)
	}

	prompt := fmt.Sprintf(interpretPrompt, fullContext)

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: prompt}}},
		},
		GenerationConfig: geminiGenConfig{
			Temperature:     0.3,
			MaxOutputTokens: 8192,
		},
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.projectID, p.region, p.model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("vertex: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("vertex status %d: %s", resp.StatusCode, string(errBody))
	}

	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("vertex decode: %w", err)
	}

	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("vertex: empty response")
	}

	return strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text), nil
}

func (p *vertexProvider) getToken() (string, error) {
	if token := os.Getenv("GOOGLE_ACCESS_TOKEN"); token != "" {
		return token, nil
	}
	out, err := exec.Command("gcloud", "auth", "print-access-token").Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth failed (install gcloud CLI or set GOOGLE_ACCESS_TOKEN): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
