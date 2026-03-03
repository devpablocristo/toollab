// Package llm provides providers and runners for docs/audit generation.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2/google"
)

// VertexProvider calls Vertex AI for LLM report generation.
type VertexProvider struct {
	projectID string
	region    string
	model     string
	http      *http.Client
}

func NewVertexProvider() *VertexProvider {
	projectID := firstNonEmptyEnv("GOOGLE_PROJECT_ID", "GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT")
	if projectID == "" {
		projectID = projectIDFromADC()
	}
	return &VertexProvider{
		projectID: projectID,
		region:    envOr("GOOGLE_REGION", "us-central1"),
		model:     envOr("GOOGLE_LLM_MODEL", "gemini-2.5-flash"),
		http:      &http.Client{Timeout: 8 * time.Minute},
	}
}

func (p *VertexProvider) Available() bool {
	if strings.TrimSpace(p.projectID) == "" {
		return false
	}
	if strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN")) != "" {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ts, err := google.DefaultTokenSource(ctx, cloudPlatformScope)
	if err != nil {
		return false
	}
	tok, err := ts.Token()
	return err == nil && tok != nil && strings.TrimSpace(tok.AccessToken) != ""
}

func (p *VertexProvider) Name() string { return "vertex-" + p.model }

// RawPrompt sends a prompt expecting JSON back (constrained decoding).
func (p *VertexProvider) RawPrompt(ctx context.Context, prompt string) ([]byte, error) {
	return p.promptWithRetries(ctx, prompt, true, 65536)
}

// TextPrompt sends a prompt expecting free-form text back (no JSON constraint).
func (p *VertexProvider) TextPrompt(ctx context.Context, prompt string) (string, error) {
	data, err := p.promptWithRetries(ctx, prompt, false, 4096)
	if err != nil {
		return "", err
	}
	return sanitizeMarkdown(string(data)), nil
}

// sanitizeMarkdown removes repetitive content that LLMs sometimes generate
// (e.g. table cells filled with thousands of repeated characters).
func sanitizeMarkdown(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	for _, line := range lines {
		if len(line) > 1000 {
			line = line[:1000] + " ..."
		}
		out = append(out, line)
	}
	result := strings.Join(out, "\n")
	const maxSize = 25000
	if len(result) > maxSize {
		result = result[:maxSize] + "\n\n*(truncated — output exceeded size limit)*"
	}
	return result
}

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

func (p *VertexProvider) promptWithRetries(ctx context.Context, prompt string, jsonMode bool, maxTokens int) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*5) * time.Second)
		}

		data, err := p.doRequest(ctx, prompt, jsonMode, maxTokens)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("after 3 attempts: %w", lastErr)
}

func (p *VertexProvider) doRequest(ctx context.Context, prompt string, jsonMode bool, maxTokens int) ([]byte, error) {
	token, err := p.accessToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.projectID, p.region, p.model,
	)

	genConfig := vertexGenConfig{
		Temperature:      0.2,
		MaxOutputTokens:  maxTokens,
		FrequencyPenalty: 0.5,
	}
	if jsonMode {
		genConfig.ResponseMimeType = "application/json"
	}

	reqBody := vertexReq{
		Contents: []vertexContent{{
			Role:  "user",
			Parts: []vertexPart{{Text: prompt}},
		}},
		GenerationConfig: genConfig,
	}

	bodyJSON, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("vertex returned %d: %s", resp.StatusCode, truncStr(string(respBody), 500))
	}

	var vresp vertexResp
	if err := json.Unmarshal(respBody, &vresp); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	if len(vresp.Candidates) == 0 || len(vresp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty LLM response")
	}

	text := vresp.Candidates[0].Content.Parts[0].Text
	text = strings.TrimSpace(text)

	if jsonMode {
		// Strip markdown code fences if present
		if strings.HasPrefix(text, "```json") {
			text = strings.TrimPrefix(text, "```json")
			if idx := strings.LastIndex(text, "```"); idx >= 0 {
				text = text[:idx]
			}
			text = strings.TrimSpace(text)
		}
		if strings.HasPrefix(text, "```") {
			text = strings.TrimPrefix(text, "```")
			if idx := strings.LastIndex(text, "```"); idx >= 0 {
				text = text[:idx]
			}
			text = strings.TrimSpace(text)
		}

		if !json.Valid([]byte(text)) {
			return nil, fmt.Errorf("LLM returned invalid JSON (len=%d)", len(text))
		}
	}

	return []byte(text), nil
}

func (p *VertexProvider) accessToken(ctx context.Context) (string, error) {
	if token := strings.TrimSpace(os.Getenv("GOOGLE_ACCESS_TOKEN")); token != "" {
		return token, nil
	}

	ts, err := google.DefaultTokenSource(ctx, cloudPlatformScope)
	if err != nil {
		return "", fmt.Errorf("loading Google default credentials: %w (set GOOGLE_ACCESS_TOKEN or configure ADC via gcloud auth application-default login)", err)
	}
	tok, err := ts.Token()
	if err != nil {
		return "", fmt.Errorf("fetching Google access token from ADC: %w", err)
	}
	if tok == nil || strings.TrimSpace(tok.AccessToken) == "" {
		return "", fmt.Errorf("google ADC returned an empty access token")
	}
	return tok.AccessToken, nil
}

type vertexReq struct {
	Contents         []vertexContent `json:"contents"`
	GenerationConfig vertexGenConfig `json:"generationConfig"`
}

type vertexContent struct {
	Role  string       `json:"role"`
	Parts []vertexPart `json:"parts"`
}

type vertexPart struct {
	Text string `json:"text"`
}

type vertexGenConfig struct {
	Temperature      float64 `json:"temperature"`
	MaxOutputTokens  int     `json:"maxOutputTokens"`
	FrequencyPenalty float64 `json:"frequencyPenalty,omitempty"`
	ResponseMimeType string  `json:"responseMimeType,omitempty"`
}

type vertexResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
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

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
