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

const (
	defaultOllamaURL = "http://localhost:11434"
	defaultModel     = "llama3.2"
	maxResponseTokens = 4096
)

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

func NewClient() *Client {
	baseURL := os.Getenv("OLLAMA_URL")
	if baseURL == "" {
		baseURL = defaultOllamaURL
	}
	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = defaultModel
	}
	timeout := 10 * time.Minute
	if t := os.Getenv("OLLAMA_TIMEOUT"); t != "" {
		if d, err := time.ParseDuration(t); err == nil {
			timeout = d
		}
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Available(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func (c *Client) Model() string { return c.model }

type generateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
	Options map[string]any `json:"options,omitempty"`
}

type generateResponse struct {
	Response string `json:"response"`
}

func (c *Client) Generate(ctx context.Context, prompt string) (string, error) {
	body := generateRequest{
		Model:  c.model,
		Prompt: prompt,
		Stream: false,
		Options: map[string]any{
			"num_predict": maxResponseTokens,
			"temperature": 0.3,
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/generate", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(errBody))
	}

	var out generateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decode ollama response: %w", err)
	}
	return strings.TrimSpace(out.Response), nil
}

// ExplainEvidence builds a prompt from evidence data and asks the LLM
// to produce a human-readable narrative. The LLM never decides PASS/FAIL.
func (c *Client) ExplainEvidence(ctx context.Context, evidenceSummary string) (string, error) {
	prompt := fmt.Sprintf(`Analiza este test de API. Escribe en español. Usa markdown.

SECCIONES REQUERIDAS:

## Resumen
Qué API, cuántos endpoints, duración, concurrencia, veredicto, tasa de éxito.

## Análisis por Flujo
Para cada endpoint de la tabla "Per-Flow Breakdown":
- **Qué es**: Qué hace el endpoint (inferido del path y método HTTP)
- **Qué se envió**: Describe el payload del request si hay sample
- **Qué respondió**: Status codes y cantidad
- **Qué debería responder**: La respuesta correcta esperada (ej: 200 con lista JSON)
- **Problema**: Si falló, la causa (path param {name} sin resolver, body inválido, etc.)

## Exitosos
Endpoints que respondieron bien.

## Fallidos
Tabla: endpoint | status recibido | esperado | causa.

## Reglas Violadas
Qué mide cada regla fallida, valor observado vs umbral.

## Recomendaciones
Acciones para mejorar.

DATOS:
%s`, evidenceSummary)

	return c.Generate(ctx, prompt)
}
