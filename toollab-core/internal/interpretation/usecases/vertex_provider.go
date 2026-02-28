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

func (p *VertexProvider) Interpret(ctx context.Context, dossier domain.Dossier, kind InterpretKind) ([]byte, error) {
	if p.projectID == "" {
		return nil, fmt.Errorf("GOOGLE_PROJECT_ID not set")
	}

	dossierJSON, err := json.MarshalIndent(dossier, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshaling dossier: %w", err)
	}

	promptTemplate := vertexAnalysisPrompt
	if kind == KindDocumentation {
		promptTemplate = vertexDocPrompt
	}

	log.Printf("vertex: kind=%s dossier=%d bytes model=%s", kind, len(dossierJSON), p.model)

	prompt := fmt.Sprintf(promptTemplate, string(dossierJSON))

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
			log.Printf("vertex: retrying %s (attempt %d)", kind, attempt+1)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
			}
		}

		result, err := p.doRequest(ctx, reqBody)
		if err != nil {
			lastErr = err
			log.Printf("vertex: %s attempt %d failed: %v", kind, attempt+1, err)
			continue
		}
		return result, nil
	}
	return nil, fmt.Errorf("vertex: all %s attempts failed: %w", kind, lastErr)
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

// NOTE: These are Go raw string literals delimited by backticks.
// You CANNOT use backticks inside them. Use single quotes instead.

// vertexDocPrompt generates API documentation: what it is, how to use it, data models, flows.
var vertexDocPrompt = `Eres un especialista en documentacion tecnica de APIs. Recibes un DOSSIER con evidencia (modelo, muestras de requests/responses, modelos inferidos) generado por una herramienta automatica de analisis. Tu tarea es producir DOCUMENTACION DE USO de la API: que es, como funciona, sus endpoints, modelos de datos y flujos de negocio.

REGLA PRINCIPAL (ANTI-ALUCINACION)
- NO INVENTES NADA. Solo usa datos del dossier.
- Si un flujo o modelo de datos no se puede reconstruir con evidencia, NO lo inventes.
- Si falta informacion, agregalo como open_question.

SALIDA
Devuelve UNICAMENTE un JSON valido con este esquema exacto:

{
  "schema_version": "v1",
  "run_id": "<copiar de dossier.run_id>",
  "overview": {
    "service_name": "Nombre del servicio",
    "description": "Resumen claro de 3-5 oraciones de lo que hace este servicio, para que sirve, cual es su dominio de negocio",
    "framework": "Framework detectado",
    "total_endpoints": 0,
    "architecture_notes": "Patrones de arquitectura, estrategia de auth, middleware, madurez REST, estructura de rutas, convenciones"
  },
  "data_models": [
    {"name": "Nombre", "description": "Que representa en el negocio", "fields": ["campo: tipo - proposito"], "used_by": ["METHOD /path"]}
  ],
  "flows": [
    {
      "name": "Nombre del flujo",
      "description": "Que hace este flujo completo",
      "importance": "critical|high|medium|low",
      "endpoints": ["METHOD /path"],
      "sequence": "Paso a paso detallado con endpoints reales...",
      "example_requests": [
        {"step": "Descripcion", "method": "POST", "path": "/example", "headers": {}, "body": {}, "expected_status": 200, "expected_response_snippet": {}, "notes": "Que observar"}
      ],
      "evidence_refs": ["<evidence_id>"]
    }
  ],
  "facts": [],
  "inferences": [],
  "open_questions": [
    {"question": "Pregunta sobre documentacion faltante", "why_missing": "Razon"}
  ],
  "guided_tour": [],
  "scenario_suggestions": []
}

IDIOMA
1) TODO el texto en ESPANOL. Campos JSON en ingles.

FUENTES PERMITIDAS
2) Solo informacion del DOSSIER:
   - service_overview / endpoints_top
   - evidence_samples (requests/responses reales)
   - analysis_summary: inferred_models, coverage, performance, behavior
   - gaps/confidence

REGLAS DE DOCUMENTACION
3) 'overview':
   - service_name: nombre descriptivo basado en la evidencia (framework, rutas, dominio).
   - description: 3-5 oraciones que expliquen QUE hace el servicio, PARA QUIEN es, y cuales son sus capacidades principales. Debe ser util para un desarrollador nuevo.
   - architecture_notes: patrones observados (REST, RPC, CRUD), middleware, auth scheme, versionado, estructura de rutas.
   - total_endpoints: del modelo.

4) 'data_models':
   - Viene de analysis_summary.inferred_models SI existen.
   - Cada modelo debe explicar su proposito de negocio, no solo listar campos.
   - fields: formato 'nombre: tipo - para que se usa'.
   - used_by: endpoints reales que usan este modelo.
   - Si no hay inferred_models, dejar [] y agregar open_question.

5) 'flows':
   - 5-10 flujos SI hay evidencia suficiente.
   - Cada flow debe representar un caso de uso real del servicio.
   - Agrupar endpoints relacionados (ej: CRUD de una entidad = un flow).
   - 'sequence': paso a paso con METHOD /path reales y que datos van de un paso al siguiente.
   - 'example_requests': usar datos REALES de evidence_samples (bodies, headers, status codes observados).
   - 'evidence_refs': al menos un evidence_id real por flow.
   - NUNCA inventes URLs, bodies o headers que no esten en la evidencia.

6) 'facts' e 'inferences': dejar como arrays vacios []. La pestaña de analisis se encarga de eso.

7) 'open_questions': 3-10 preguntas honestas sobre lo que NO se pudo documentar:
   - Autenticacion no clara, roles, rate limits, webhooks, etc.

FORMATO FINAL
8) Solo JSON valido y parseable. Sin backticks, sin markdown.

DOSSIER:
%s`

// vertexAnalysisPrompt interprets the audit: security, behavior, facts, inferences, improvements, tests.
var vertexAnalysisPrompt = `Eres un auditor senior de seguridad (AppSec) y analista de calidad de APIs. Recibes un DOSSIER con evidencia (auditoria determinista, muestras de requests/responses, metricas) generado por una herramienta automatica. Tu tarea es INTERPRETAR la auditoria: analizar seguridad, comportamiento, extraer hechos, inferencias, mejoras y tests.

REGLA PRINCIPAL (ANTI-ALUCINACION)
- NO INVENTES NADA.
- Toda afirmacion debe estar anclada a evidencia del dossier mediante 'evidence_refs'.
- Si no hay evidencia suficiente, expresarlo como open_question.

SALIDA
Devuelve UNICAMENTE un JSON valido con este esquema exacto:

{
  "schema_version": "v1",
  "run_id": "<copiar de dossier.run_id>",
  "overview": null,
  "data_models": [],
  "flows": [],
  "security_assessment": {
    "overall_risk": "critical|high|medium|low",
    "summary": "Resumen de 2-3 oraciones sobre la postura de seguridad",
    "critical_findings": ["Hallazgo 1 explicado claramente"],
    "positive_findings": ["Lo que la API hace bien"],
    "attack_surface": "Descripcion de la superficie de ataque"
  },
  "behavior_assessment": {
    "input_validation": "Que tan bien valida la entrada?",
    "auth_enforcement": "Como funciona la autenticacion?",
    "error_handling": "Que tan consistentes son los errores?",
    "robustness": "Como maneja casos extremos?"
  },
  "facts": [
    {"id": "fact_001", "text": "Observacion factual", "evidence_refs": ["<evidence_id>"], "confidence": 0.95}
  ],
  "inferences": [
    {"id": "inf_001", "text": "Conclusion logica", "rule_of_inference": "nombre_regla", "evidence_refs": ["<evidence_id>"], "confidence": 0.8}
  ],
  "improvements": [
    {"title": "Mejora", "severity": "critical|high|medium|low", "category": "security|contract|performance|reliability", "description": "Que esta mal", "remediation": "Como arreglarlo", "evidence_refs": ["<evidence_id>"]}
  ],
  "tests": [
    {"name": "Test", "description": "Que verifica", "flow": "Flujo", "importance": "critical|high|medium|low", "request": {"method": "GET", "path": "/x"}, "expected": {"status": 200, "description": "Respuesta esperada"}, "evidence_refs": ["<evidence_id>"]}
  ],
  "open_questions": [
    {"question": "Pregunta", "why_missing": "Razon"}
  ],
  "guided_tour": [],
  "scenario_suggestions": []
}

IDIOMA
1) TODO el texto en ESPANOL. Campos JSON en ingles.

FUENTES PERMITIDAS
2) Solo informacion del DOSSIER:
   - audit_highlights (findings deterministas)
   - evidence_samples (requests/responses)
   - analysis_summary: security, contract, behavior, performance
   - gaps/confidence

REGLAS DE TRAZABILIDAD
3) Facts, Inferences, Improvements: SIEMPRE 'evidence_refs' a IDs reales de dossier.evidence_samples.
4) Distribuir evidencia: usar 5-12 evidence_ids distintos si existen.

REGLAS DE SEGURIDAD
5) 'security_assessment':
   - Priorizar findings del audit (AUTH_MISSING_2XX, ERROR_LEAK_STACKTRACE, etc.).
   - Si no hay evidencia de auth, NO asumir que tiene auth: poner open_question.
   - overall_risk: determinado por el peor hallazgo observado.
   - critical_findings: explicar cada hallazgo critico/alto en lenguaje claro.
   - positive_findings: que hace bien la API (validacion, headers, etc.).
   - attack_surface: endpoints expuestos, auth gaps, datos sensibles.

6) 'behavior_assessment':
   - Basarse en evidencia: status codes, mensajes de error, content-type, latencias.
   - Si no hay datos, decirlo.

REGLAS DE CALIDAD
7) 'facts': 8-20 hechos verificables.
   - Ej: "El endpoint POST /users respondio 200 sin header Authorization".
   - NO usar lenguaje vago en facts.

8) 'inferences': 5-12 inferencias con regla explicita.
   - rule_of_inference: nombre descriptivo (ej: "2xx_without_auth_implies_missing_enforcement").

9) 'improvements': 6-15 mejoras accionables.
   - remediation concreta: "Agregar middleware JWT en router X", "Sanitizar errores 500".
   - Cada improvement con severity y category.

10) 'tests': 6-15 tests sugeridos.
    - Mapear a mejoras e inferencias. Usar endpoints reales del dossier.
    - Explicar POR QUE importa cada test.

11) 'open_questions': 5-20 preguntas sobre seguridad/comportamiento desconocido.

REGLAS DE CONSISTENCIA
12) No contradigas la evidencia.
13) Anomalias (contract_anomaly): presentar como "anomalia observada", no bug confirmado.

14) 'overview', 'data_models', 'flows': dejar null/vacios. La pestaña de documentacion se encarga.

FORMATO FINAL
15) Solo JSON valido. Sin backticks, sin markdown.

DOSSIER:
%s`
