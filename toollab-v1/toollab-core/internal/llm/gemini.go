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

// geminiProvider uses the Google AI Studio REST API (generativelanguage.googleapis.com).
// Only requires GEMINI_API_KEY. Supports Gemini 2.5 Flash, 2.5 Pro, etc.
type geminiProvider struct {
	apiKey string
	model  string
	http   *http.Client
}

func newGeminiProvider() *geminiProvider {
	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &geminiProvider{
		apiKey: os.Getenv("GEMINI_API_KEY"),
		model:  model,
		http:   &http.Client{Timeout: 2 * time.Minute},
	}
}

func (p *geminiProvider) Name() string {
	return fmt.Sprintf("gemini/%s", p.model)
}

func (p *geminiProvider) Available(ctx context.Context) bool {
	return p.apiKey != ""
}

func (p *geminiProvider) Interpret(ctx context.Context, fullContext string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not set")
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

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		p.model, p.apiKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return "", fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(errBody))
	}

	var out geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("gemini decode: %w", err)
	}

	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}

	return strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text), nil
}

type geminiRequest struct {
	Contents         []geminiContent  `json:"contents"`
	GenerationConfig geminiGenConfig  `json:"generationConfig"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	Temperature     float64 `json:"temperature"`
	MaxOutputTokens int     `json:"maxOutputTokens"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
