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

// NOTE: This is a Go raw string literal delimited by backticks.
// You CANNOT use backticks inside it. Use single quotes instead for emphasis.
var vertexInterpretPrompt = `Eres un analista senior de APIs, auditor de seguridad (AppSec) y especialista en documentacion tecnica. Recibes un DOSSIER con evidencia (modelo, auditoria determinista y muestras de requests/responses) generado por una herramienta automatica. Tu tarea es producir DOCUMENTACION VIVIENTE + AUDITORIA INTERPRETADA de una API HTTP.

REGLA PRINCIPAL (ANTI-ALUCINACION)
- NO INVENTES NADA.
- Toda afirmacion factual debe estar anclada a evidencia del dossier mediante 'evidence_refs' y/o a hallazgos deterministas mediante 'finding_refs' (si el dossier los provee).
- Si no hay evidencia suficiente para afirmar algo, debes expresarlo como open_question con 'why_missing'.
- Si un flujo o modelo de datos no se puede reconstruir con evidencia, NO lo inventes: incluyelo solo si esta soportado por 'analysis_summary.inferred_models' o por evidencia concreta (requests/responses).

SALIDA
Devuelve UNICAMENTE un JSON valido (sin markdown, sin texto extra) que cumpla EXACTAMENTE este esquema (no agregues campos nuevos, no cambies nombres de campos):

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
    {"name": "Nombre del modelo", "description": "Que representa en el dominio de negocio", "fields": ["campo: tipo - proposito"], "used_by": ["METHOD /path"]}
  ],
  "flows": [
    {
      "name": "Nombre del flujo de negocio",
      "description": "Que hace este flujo de principio a fin",
      "importance": "critical|high|medium|low",
      "endpoints": ["METHOD /path"],
      "sequence": "Paso a paso detallado...",
      "example_requests": [
        {"step": "Descripcion del paso", "method": "POST", "path": "/example", "headers": {}, "body": {}, "expected_status": 200, "expected_response_snippet": {}, "notes": "Que observar en esta respuesta"}
      ],
      "evidence_refs": ["<evidence_id del dossier>"]
    }
  ],
  "security_assessment": {
    "overall_risk": "critical|high|medium|low",
    "summary": "Resumen de 2-3 oraciones sobre la postura de seguridad",
    "critical_findings": ["Hallazgo 1 explicado claramente"],
    "positive_findings": ["Lo que la API hace bien en seguridad"],
    "attack_surface": "Descripcion de la superficie de ataque"
  },
  "behavior_assessment": {
    "input_validation": "Que tan bien valida la API la entrada?",
    "auth_enforcement": "Como funciona la autenticacion?",
    "error_handling": "Que tan consistentes e informativos son los errores?",
    "robustness": "Como maneja la API casos extremos, payloads grandes, content types incorrectos?"
  },
  "facts": [
    {"id": "fact_001", "text": "Observacion factual en espanol", "evidence_refs": ["<evidence_id>"], "confidence": 0.95}
  ],
  "inferences": [
    {"id": "inf_001", "text": "Conclusion logica en espanol", "rule_of_inference": "nombre_regla", "evidence_refs": ["<evidence_id>"], "confidence": 0.8}
  ],
  "improvements": [
    {"title": "Mejora especifica", "severity": "critical|high|medium|low", "category": "security|contract|performance|reliability", "description": "Que esta mal y por que importa", "remediation": "Pasos concretos para arreglarlo", "evidence_refs": ["<evidence_id>"]}
  ],
  "tests": [
    {"name": "Nombre del test", "description": "Que verifica este test y POR QUE importa", "flow": "Que flujo", "importance": "critical|high|medium|low", "request": {"method": "GET", "path": "/x"}, "expected": {"status": 200, "description": "Como deberia verse la respuesta correcta"}, "evidence_refs": ["<evidence_id>"]}
  ],
  "open_questions": [
    {"question": "Lo que no se pudo determinar", "why_missing": "Razon"}
  ],
  "guided_tour": [],
  "scenario_suggestions": []
}

IDIOMA
1) TODO EL TEXTO de los valores debe estar en ESPANOL (descriptions, summaries, notes, etc.). Los nombres de campos JSON quedan en ingles.

FUENTES PERMITIDAS (SOLO ESTO)
2) Solo podes usar informacion del DOSSIER provisto:
   - service_overview / endpoints_top
   - audit_highlights (findings deterministas)
   - evidence_samples (requests/responses)
   - analysis_summary (si esta incluido): inferred_models, coverage, performance, contract, behavior, security findings
   - gaps/confidence
Si el dossier no contiene analysis_summary, igual producis salida, pero con mas open_questions.

REGLAS DE TRAZABILIDAD
3) Para TODO lo importante:
   - Facts e Inferences: SIEMPRE 'evidence_refs' a IDs reales de dossier.evidence_samples.
   - Improvements: SIEMPRE 'evidence_refs' (o el evidence_id asociado al finding del audit_highlight).
   - Flows: SIEMPRE 'evidence_refs' con evidencia real; NO inventes payloads.
4) No repitas el mismo evidence_id en todo. Distribui evidencia: idealmente 5-12 evidence_ids distintos si existen.

REGLAS DE COBERTURA (EXHAUSTIVO PERO HONESTO)
5) 'overview.total_endpoints' debe salir del modelo (endpoints_count o longitud de endpoints_top si es lo unico disponible).
6) 'data_models' debe venir de analysis_summary.inferred_models. Si no hay inferred_models, dejar 'data_models: []' y agregar open_question sobre modelos faltantes.
7) 'flows':
   - Incluye de 3 a 8 flows SI hay evidencia suficiente.
   - Un flow debe estar soportado por evidencia de multiples endpoints (idealmente 2+ steps). Si solo hay un endpoint, declaralo como flujo simple.
   - 'sequence' debe describir pasos con endpoints reales (METHOD /path) y explicar dependencias.
   - 'example_requests': usar datos reales de evidence_samples. Si el body real no esta completo, usar el snippet real y aclarar en notes que es truncado.
8) 'security_assessment':
   - Prioriza findings del audit (AUTH_MISSING_2XX, ERROR_LEAK_STACKTRACE, etc.) si estan presentes.
   - Si no hay evidencia de auth, no asumas "tiene auth": pon open_question.
   - overall_risk se determina por el peor hallazgo observado (si hay leak o auth gap, tender a high/critical segun evidencia).
9) 'behavior_assessment':
   - Basate en evidencia: status codes, mensajes de error, content-type, tamanos, latencias (si estan).
   - Si no hay datos, decirlo y poner open_question.

REGLAS DE ESTRUCTURA / CALIDAD
10) 'facts': 8-20 hechos (si hay evidencia suficiente).
   - Cada fact debe ser una observacion verificable: "El endpoint X respondio 200 sin Authorization", "Se observo body con panic en /debug/error".
11) 'inferences': 5-12 inferencias.
   - Cada inference debe tener una regla explicita en rule_of_inference (ej: "2xx_without_auth_implies_missing_auth_enforcement").
12) 'improvements': 6-15 mejoras accionables.
   - Remediations deben ser concretas (ej: "Agregar middleware JWT en router X", "Sanitizar errores 500", "Agregar limites de payload").
13) 'tests': 6-15 tests sugeridos.
   - Deben mapear flows y mejoras. Deben explicar por que importan.
   - Usar endpoints reales del dossier, no inventados.
14) 'open_questions': 5-20 preguntas honestas si faltan datos (auth scheme, rate limits, roles, idempotencia, etc.)

REGLAS DE CONSISTENCIA
15) No contradigas la evidencia. Si un finding dice "auth missing 2xx", tu texto debe reflejarlo.
16) Si audit marca algo como "anomalia" (contract_anomaly), NO lo presentes como bug confirmado. Presentalo como "anomalia observada" y sugeri experimentos/tests.
17) No uses lenguaje vago ("podria", "quizas") en facts; reserva eso para inferences u open_questions.

GUIDED_TOUR y SCENARIO_SUGGESTIONS
18) Deben devolverse como arrays vacios [] (por requerimiento).

FORMATO FINAL
19) La respuesta debe ser SOLO el JSON, valido y parseable.
20) No incluyas backticks, no incluyas explicaciones fuera del JSON.

DOSSIER:
%s`
