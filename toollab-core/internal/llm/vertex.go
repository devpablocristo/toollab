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
)

// VertexProvider calls Vertex AI for LLM report generation.
type VertexProvider struct {
	projectID string
	region    string
	model     string
	http      *http.Client
}

func NewVertexProvider() *VertexProvider {
	return &VertexProvider{
		projectID: os.Getenv("GOOGLE_PROJECT_ID"),
		region:    envOr("GOOGLE_REGION", "us-central1"),
		model:     envOr("GOOGLE_LLM_MODEL", "gemini-2.5-flash"),
		http:      &http.Client{Timeout: 8 * time.Minute},
	}
}

func (p *VertexProvider) Available() bool {
	return p.projectID != "" && (os.Getenv("GOOGLE_ACCESS_TOKEN") != "" || os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") != "")
}

func (p *VertexProvider) Name() string { return "vertex-" + p.model }

// RawPrompt sends a prompt expecting JSON back (constrained decoding).
func (p *VertexProvider) RawPrompt(ctx context.Context, prompt string) ([]byte, error) {
	return p.promptWithRetries(ctx, prompt, true)
}

// TextPrompt sends a prompt expecting free-form text back (no JSON constraint).
func (p *VertexProvider) TextPrompt(ctx context.Context, prompt string) (string, error) {
	data, err := p.promptWithRetries(ctx, prompt, false)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (p *VertexProvider) promptWithRetries(ctx context.Context, prompt string, jsonMode bool) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt*5) * time.Second)
		}

		data, err := p.doRequest(ctx, prompt, jsonMode)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("after 3 attempts: %w", lastErr)
}

func (p *VertexProvider) doRequest(ctx context.Context, prompt string, jsonMode bool) ([]byte, error) {
	token := os.Getenv("GOOGLE_ACCESS_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GOOGLE_ACCESS_TOKEN not set")
	}

	url := fmt.Sprintf(
		"https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.projectID, p.region, p.model,
	)

	genConfig := vertexGenConfig{
		Temperature:     0.2,
		MaxOutputTokens: 65536,
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

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
