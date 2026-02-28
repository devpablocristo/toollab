package usecases

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"toollab-core/internal/interpretation/usecases/domain"
)

type VertexProvider struct {
	projectID string
	region    string
	model     string
	http      *http.Client
}

func NewVertexProvider() *VertexProvider {
	region := os.Getenv("GOOGLE_REGION")
	if region == "" {
		region = "us-central1"
	}
	model := os.Getenv("VERTEX_MODEL")
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &VertexProvider{
		projectID: os.Getenv("GOOGLE_PROJECT_ID"),
		region:    region,
		model:     model,
		http:      &http.Client{Timeout: 5 * time.Minute},
	}
}

func (p *VertexProvider) Name() string {
	return fmt.Sprintf("vertex/%s", p.model)
}

func (p *VertexProvider) Available() bool {
	if p.projectID == "" {
		return false
	}
	_, err := p.getToken()
	return err == nil
}

func (p *VertexProvider) Interpret(ctx context.Context, dossier domain.Dossier) ([]byte, error) {
	if p.projectID == "" {
		return nil, fmt.Errorf("GOOGLE_PROJECT_ID not set")
	}

	dossierJSON, err := json.MarshalIndent(dossier, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling dossier: %w", err)
	}

	log.Printf("vertex: dossier size = %d bytes, sending to %s", len(dossierJSON), p.model)

	prompt := fmt.Sprintf(vertexInterpretPrompt, string(dossierJSON))

	reqBody := vertexRequest{
		Contents: []vertexContent{
			{Role: "user", Parts: []vertexPart{{Text: prompt}}},
		},
		GenerationConfig: vertexGenConfig{
			Temperature:      0.3,
			MaxOutputTokens:  65536,
			ResponseMimeType: "application/json",
		},
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			log.Printf("vertex: retrying (attempt %d)", attempt+1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}

		result, err := p.doRequest(ctx, reqBody)
		if err != nil {
			lastErr = err
			log.Printf("vertex: attempt %d failed: %v", attempt+1, err)
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("vertex: all attempts failed: %w", lastErr)
}

func (p *VertexProvider) doRequest(ctx context.Context, reqBody vertexRequest) ([]byte, error) {
	token, err := p.getToken()
	if err != nil {
		return nil, fmt.Errorf("vertex auth: %w", err)
	}

	raw, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		p.region, p.projectID, p.region, p.model)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := p.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, string(errBody))
	}

	var out vertexResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	text := strings.TrimSpace(out.Candidates[0].Content.Parts[0].Text)
	text = stripJSONFence(text)

	log.Printf("vertex: response length = %d chars", len(text))

	var interp domain.LLMInterpretation
	if err := json.Unmarshal([]byte(text), &interp); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nraw: %s", err, truncateStr(text, 500))
	}

	return json.Marshal(interp)
}

func (p *VertexProvider) getToken() (string, error) {
	if token := os.Getenv("GOOGLE_ACCESS_TOKEN"); token != "" {
		return token, nil
	}
	out, err := exec.Command("gcloud", "auth", "print-access-token").Output()
	if err != nil {
		return "", fmt.Errorf("gcloud auth failed (set GOOGLE_ACCESS_TOKEN): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func stripJSONFence(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

type vertexRequest struct {
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

type vertexResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

const vertexInterpretPrompt = `Eres un analista experto en APIs y auditor de seguridad. Recibes un DOSSIER con evidencia del análisis automatizado de una API HTTP.

Produce DOCUMENTACIÓN VIVIENTE COMPLETA en español como JSON válido siguiendo EXACTAMENTE este esquema:

{
  "schema_version": "v1",
  "run_id": "<copiar de dossier.run_id>",
  "overview": {
    "service_name": "Nombre del servicio",
    "description": "Resumen de 2-3 oraciones de lo que hace este servicio",
    "framework": "Framework detectado",
    "total_endpoints": 0,
    "architecture_notes": "Patrones de arquitectura, estrategia de auth, middleware, madurez REST, flujo de datos"
  },
  "data_models": [
    {"name": "Nombre del modelo", "description": "Qué representa en el dominio de negocio", "fields": ["campo: tipo - propósito"], "used_by": ["METHOD /path"]}
  ],
  "flows": [
    {
      "name": "Nombre del flujo de negocio", "description": "Qué hace este flujo de principio a fin", "importance": "critical|high|medium|low",
      "endpoints": ["METHOD /path"],
      "sequence": "Paso a paso detallado: 1. El cliente envía POST /x con {...} 2. El servidor valida y crea... 3. El cliente puede hacer GET /x/:id para obtener...",
      "example_requests": [
        {"step": "Descripción del paso", "method": "POST", "path": "/example", "headers": {}, "body": {}, "expected_status": 200, "expected_response_snippet": {}, "notes": "Qué observar en esta respuesta"}
      ],
      "evidence_refs": ["<evidence_id del dossier>"]
    }
  ],
  "security_assessment": {
    "overall_risk": "critical|high|medium|low",
    "summary": "Resumen de 2-3 oraciones sobre la postura de seguridad",
    "critical_findings": ["Hallazgo 1 explicado claramente"],
    "positive_findings": ["Lo que la API hace bien en seguridad"],
    "attack_surface": "Descripción de la superficie de ataque"
  },
  "behavior_assessment": {
    "input_validation": "¿Qué tan bien valida la API la entrada?",
    "auth_enforcement": "¿Cómo funciona la autenticación?",
    "error_handling": "¿Qué tan consistentes e informativos son los errores?",
    "robustness": "¿Cómo maneja la API casos extremos, payloads grandes, content types incorrectos?"
  },
  "facts": [
    {"id": "fact_001", "text": "Observación factual en español", "evidence_refs": ["<evidence_id>"], "confidence": 0.95}
  ],
  "inferences": [
    {"id": "inf_001", "text": "Conclusión lógica en español", "rule_of_inference": "nombre_regla", "evidence_refs": ["<evidence_id>"], "confidence": 0.8}
  ],
  "improvements": [
    {"title": "Mejora específica", "severity": "critical|high|medium|low", "category": "security|contract|performance|reliability", "description": "Qué está mal y por qué importa", "remediation": "Pasos concretos para arreglarlo", "evidence_refs": ["<evidence_id>"]}
  ],
  "tests": [
    {"name": "Nombre del test", "description": "Qué verifica este test y POR QUÉ importa", "flow": "Qué flujo", "importance": "critical|high|medium|low", "request": {"method": "GET", "path": "/x"}, "expected": {"status": 200, "description": "Cómo debería verse la respuesta correcta"}, "evidence_refs": ["<evidence_id>"]}
  ],
  "open_questions": [
    {"question": "Lo que no se pudo determinar", "why_missing": "Razón"}
  ],
  "guided_tour": [],
  "scenario_suggestions": []
}

REGLAS:
1. TODO EL TEXTO debe estar en ESPAÑOL. Los nombres de campos JSON permanecen en inglés, pero todos los valores de texto (description, summary, text, etc.) deben ser en español.
2. USA los datos de analysis_summary: hallazgos de seguridad, análisis de comportamiento, modelos inferidos, rendimiento, cobertura, contrato.
3. FLOWS: incluir example_requests con datos reales de request/response de evidence_samples. Mostrar payloads reales.
4. DATA_MODELS: documentar los inferred_models del analysis_summary.
5. facts/inferences: cada uno DEBE tener un campo "id" (ej: "fact_001") y referenciar evidence_ids reales de dossier.evidence_samples.
6. guided_tour y scenario_suggestions: devolver arrays vacíos [].
7. Sé exhaustivo. Esta es la documentación definitiva de esta API.

DOSSIER:
%s`
