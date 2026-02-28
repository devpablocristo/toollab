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
		model = "gemini-2.5-pro"
	}
	return &VertexProvider{
		projectID: os.Getenv("GOOGLE_PROJECT_ID"),
		region:    region,
		model:     model,
		http:      &http.Client{Timeout: 8 * time.Minute},
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
	for attempt := 0; attempt < 3; attempt++ {
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

	sanitized, err := sanitizeLLMResponse([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %w\nraw: %s", err, truncateStr(text, 500))
	}

	var interp domain.LLMInterpretation
	if err := json.Unmarshal(sanitized, &interp); err != nil {
		return nil, fmt.Errorf("invalid JSON after sanitize: %w\nraw: %s", err, truncateStr(string(sanitized), 500))
	}

	return json.Marshal(interp)
}

// sanitizeLLMResponse fixes common LLM output issues: fields that should be
// arrays of objects but come back as strings or arrays of strings.
func sanitizeLLMResponse(raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	objectArrayFields := []string{"facts", "inferences", "improvements", "tests", "open_questions", "data_models", "flows", "guided_tour", "scenario_suggestions"}
	for _, field := range objectArrayFields {
		v, ok := m[field]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			m[field] = []any{}
		case []any:
			var cleaned []any
			for _, item := range val {
				switch item.(type) {
				case map[string]any:
					cleaned = append(cleaned, item)
				case string:
					// skip string entries in object arrays
				default:
					cleaned = append(cleaned, item)
				}
			}
			if cleaned == nil {
				cleaned = []any{}
			}
			m[field] = cleaned
		case nil:
			m[field] = []any{}
		default:
			_ = val
			m[field] = []any{}
		}
	}

	nullableFields := []string{"overview", "security_assessment", "behavior_assessment"}
	for _, field := range nullableFields {
		v, ok := m[field]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			m[field] = nil
		}
	}

	return json.Marshal(m)
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

// vertexDocPrompt generates rich, human-readable API documentation.
var vertexDocPrompt = `Eres un escritor tecnico senior especializado en documentacion de APIs. Tu audiencia son DESARROLLADORES HUMANOS que necesitan entender una API que nunca vieron antes. Recibes un DOSSIER generado por una herramienta automatica con evidencia real (endpoints, requests/responses, modelos inferidos, metricas).

TU MISION: Producir documentacion que un humano pueda leer y decir "ahora entiendo perfectamente que hace esta API, como funciona, y como la uso". No es una lista tecnica de campos — es una EXPLICACION NARRATIVA.

REGLA DE ORO: NO INVENTES. Todo debe venir del dossier. Si no hay datos suficientes, dilo en open_questions.

SALIDA: JSON valido con este esquema exacto:

{
  "schema_version": "v1",
  "run_id": "<copiar de dossier.run_id>",
  "overview": {
    "service_name": "Nombre claro del servicio",
    "description": "PARRAFO LARGO (8-15 oraciones) que explique: (1) Que ES este servicio y cual es su proposito de negocio. (2) Que problema resuelve y para quien. (3) Cuales son sus capacidades principales (CRUD de que entidades, que operaciones soporta). (4) Como esta organizado (grupos de rutas, versionado, patrones). (5) Que tipo de aplicaciones lo consumirian. Escribi como si le explicaras a un desarrollador nuevo que se acaba de sumar al equipo.",
    "framework": "Framework detectado",
    "total_endpoints": 0,
    "architecture_notes": "PARRAFO DETALLADO (5-10 oraciones) sobre: patrones de arquitectura (REST puro, RPC, CRUD), como estan organizadas las rutas (prefijos, versionado, agrupacion por recurso), que middleware se observa (auth, CORS, logging, recovery), como maneja errores (formato, consistencia), como maneja content types, si tiene health checks, si usa paginacion, si tiene API keys o JWT o nada. Basate en la evidencia real."
  },
  "data_models": [
    {
      "name": "Nombre del modelo",
      "description": "PARRAFO (3-5 oraciones) explicando: que representa este modelo en el dominio de negocio, para que se usa, como se relaciona con otros modelos, que rol juega en los flujos del servicio. NO es solo una lista de campos — es una explicacion de negocio.",
      "fields": ["nombre: tipo - explicacion clara de para que sirve este campo y que valores tipicos tiene"],
      "used_by": ["METHOD /path - que operacion hace con este modelo"]
    }
  ],
  "flows": [
    {
      "name": "Nombre descriptivo del caso de uso",
      "description": "PARRAFO (5-8 oraciones) que cuente la HISTORIA COMPLETA del flujo: que quiere lograr el usuario/cliente, por que necesita hacerlo, que pasa en cada paso, que datos fluyen de un endpoint al otro, que esperar como respuesta, y que puede salir mal. Debe leerse como una historia, no como una lista.",
      "importance": "critical|high|medium|low",
      "endpoints": ["METHOD /path"],
      "sequence": "NARRATIVA DETALLADA paso a paso. Ejemplo: 'Primero, el cliente envia un POST a /api/v1/users con el body {name, email} para crear un usuario. El servidor responde con 201 y devuelve el objeto creado incluyendo el ID asignado. Luego, con ese ID, el cliente puede hacer GET /api/v1/users/{id} para obtener los detalles completos. Si necesita actualizar, envia PUT /api/v1/users/{id} con los campos modificados...' USA DATOS REALES de la evidencia.",
      "example_requests": [
        {"step": "Descripcion clara de que hace este paso y POR QUE", "method": "POST", "path": "/example", "headers": {}, "body": {}, "expected_status": 200, "expected_response_snippet": {}, "notes": "Que observar en la respuesta: que campos son importantes, que indica el status code, que headers revisar"}
      ],
      "evidence_refs": ["<evidence_id>"]
    }
  ],
  "facts": [],
  "inferences": [],
  "open_questions": [
    {"question": "Pregunta clara sobre algo que un desarrollador necesitaria saber pero no se pudo determinar", "why_missing": "Explicacion de por que no se pudo determinar y que evidencia faltaria"}
  ],
  "guided_tour": [],
  "scenario_suggestions": []
}

IDIOMA
1) TODO el texto de los valores en ESPANOL. Campos JSON en ingles.

FUENTES PERMITIDAS
2) Solo informacion del DOSSIER. NUNCA inventes URLs, bodies, headers o datos que no esten en la evidencia.

CALIDAD DE ESCRITURA — ESTO ES LO MAS IMPORTANTE
3) Escribi como un HUMANO EXPERTO que explica a otro humano. NO como una maquina que llena campos.
   - MALO: "Servicio REST con endpoints CRUD para usuarios."
   - BUENO: "Este servicio es una API REST construida con el framework Gin que gestiona el ciclo de vida completo de usuarios y sus recursos asociados. Expone 62 endpoints organizados en grupos por recurso (/users, /projects, /tasks), soporta autenticacion via API key en el header X-API-Key, y utiliza JSON como formato exclusivo de intercambio. El servicio parece ser el backend principal de una aplicacion de gestion de proyectos, donde los usuarios pueden crear proyectos, asignar tareas, y gestionar permisos de equipo."

4) overview.description: MINIMO 8 oraciones. Debe ser un parrafo completo que un humano nuevo pueda leer y entender todo el servicio de un vistazo. Incluye: dominio de negocio, entidades principales, capacidades, organizacion, y para que tipo de aplicacion sirve.

5) overview.architecture_notes: MINIMO 5 oraciones. Explica patrones, middleware, auth, organizacion de rutas, manejo de errores, content types. No solo listes — explica QUE SIGNIFICA cada patron observado.

6) data_models: Cada description debe ser un PARRAFO que explique el rol de negocio del modelo, no solo "modelo de usuario". Explica QUE representa, COMO se usa, y POR QUE existe.

7) flows: ESTA ES LA SECCION MAS IMPORTANTE.
   - Genera entre 5 y 12 flujos.
   - Cada flow debe contar una HISTORIA: "Un cliente quiere hacer X. Para eso, primero necesita Y, luego Z..."
   - 'description': 5-8 oraciones minimo. Explica el caso de uso completo, no solo que endpoints toca.
   - 'sequence': Narrativa paso a paso con datos reales. Usa METHOD /path concretos y explica que datos van y vienen.
   - 'example_requests': Usa bodies y headers REALES de evidence_samples. Si un body esta truncado, aclara en notes.
   - Agrupa endpoints relacionados. Un CRUD completo es UN flow ("Gestion de usuarios: crear, leer, actualizar, eliminar").
   - Incluye flujos de error: "Que pasa si el recurso no existe? Que pasa sin autenticacion?"

8) open_questions: 5-15 preguntas que un desarrollador nuevo haria: roles, permisos, rate limits, webhooks, formatos de paginacion, versionado, ambientes, SDKs, etc.

FORMATO FINAL
9) Solo JSON valido. Sin backticks, sin markdown, sin texto fuera del JSON.

DOSSIER:
%s`

// vertexAnalysisPrompt interprets the audit with rich, human-readable explanations.
var vertexAnalysisPrompt = `Eres un auditor senior de seguridad (AppSec) y analista de calidad de APIs con 15 anos de experiencia. Tu audiencia son DESARROLLADORES HUMANOS y LIDERES TECNICOS que necesitan entender el estado de seguridad y calidad de su API. Recibes un DOSSIER con evidencia real.

TU MISION: Producir un ANALISIS INTERPRETADO que un humano pueda leer y decir "ahora entiendo exactamente que problemas tiene mi API, por que son graves, y como los arreglo". No es una lista de codigos — es un DIAGNOSTICO EXPERTO.

REGLA DE ORO: NO INVENTES. Todo anclado a evidence_refs reales del dossier. Si no hay datos, ponelo en open_questions.

SALIDA: JSON valido con este esquema exacto:

{
  "schema_version": "v1",
  "run_id": "<copiar de dossier.run_id>",
  "overview": null,
  "data_models": [],
  "flows": [],
  "security_assessment": {
    "overall_risk": "critical|high|medium|low",
    "summary": "PARRAFO DE 5-8 ORACIONES que explique la postura de seguridad COMO UN CONSULTOR le explicaria al CTO. No solo 'tiene problemas de auth' — explica QUE problemas, POR QUE son graves, QUE impacto tienen, y CUAL es la prioridad de arreglo. Ejemplo: 'La API presenta una superficie de ataque significativa debido a que el 80% de los endpoints responden con 200 OK sin requerir ningun tipo de autenticacion. Esto significa que cualquier persona con acceso a la red puede leer, crear y modificar datos sin restriccion alguna. Se observaron ademas stack traces completos del framework en respuestas de error, lo cual expone informacion interna del servidor (rutas de archivos, versiones de librerias) que un atacante podria usar para planificar ataques dirigidos...'",
    "critical_findings": ["ORACION COMPLETA explicando el hallazgo, su impacto concreto, y por que importa. No solo 'falta auth' — explica 'Se observo que 45 de 62 endpoints responden 200 OK sin header Authorization, lo que permite acceso no autenticado a operaciones de creacion y modificacion de datos'"],
    "positive_findings": ["ORACION COMPLETA explicando que hace bien la API y por que importa"],
    "attack_surface": "PARRAFO DE 3-5 ORACIONES describiendo la superficie de ataque: cuantos endpoints estan expuestos, que datos sensibles se pueden acceder, que operaciones destructivas estan disponibles sin auth, que informacion se filtra en errores"
  },
  "behavior_assessment": {
    "input_validation": "PARRAFO DE 3-5 ORACIONES: Como reacciona la API cuando recibe datos invalidos, malformados, o inesperados? Da ejemplos concretos observados. Ejemplo: 'Cuando se envia un JSON malformado a POST /users, la API responde con 400 y un mensaje de error claro indicando el campo problematico. Sin embargo, cuando se envian campos extra no esperados, la API los ignora silenciosamente sin advertencia...'",
    "auth_enforcement": "PARRAFO DE 3-5 ORACIONES: Como funciona (o no funciona) la autenticacion? Se observo algun mecanismo? Que pasa cuando se accede sin credenciales? Ejemplo: 'No se observo ningun mecanismo de autenticacion activo. Todos los endpoints probados respondieron exitosamente sin headers de autorizacion. Esto incluye operaciones sensibles como DELETE y PUT que modifican o eliminan datos...'",
    "error_handling": "PARRAFO DE 3-5 ORACIONES: Que tan consistentes y seguros son los mensajes de error? Se filtran stack traces? Los codigos de error son correctos? Ejemplo: 'Los errores son inconsistentes: algunos endpoints devuelven 404 con un JSON estructurado {error: mensaje}, mientras otros devuelven 500 con el stack trace completo del framework incluyendo rutas de archivos del servidor...'",
    "robustness": "PARRAFO DE 3-5 ORACIONES: Como maneja payloads grandes, content types incorrectos, metodos HTTP no soportados? Da ejemplos observados."
  },
  "facts": [
    {"id": "fact_001", "text": "ORACION COMPLETA Y ESPECIFICA con datos concretos. No 'endpoint sin auth' sino 'El endpoint POST /api/v1/users respondio 200 OK con body conteniendo datos del usuario creado, sin requerir ningun header de Authorization ni API key, lo que confirma que la creacion de usuarios esta completamente desprotegida'", "evidence_refs": ["<evidence_id>"], "confidence": 0.95}
  ],
  "inferences": [
    {"id": "inf_001", "text": "ORACION COMPLETA explicando la conclusion y su razonamiento. No 'posible falta de auth' sino 'Dado que 45 de 62 endpoints responden exitosamente sin autenticacion, y que esto incluye operaciones de escritura (POST, PUT, DELETE), se infiere que el servicio no tiene implementado un middleware de autenticacion a nivel de router, o que esta desactivado en el ambiente de prueba'", "rule_of_inference": "nombre_descriptivo_de_la_regla", "evidence_refs": ["<evidence_id>"], "confidence": 0.8}
  ],
  "improvements": [
    {"title": "Titulo claro de la mejora", "severity": "critical|high|medium|low", "category": "security|contract|performance|reliability", "description": "PARRAFO DE 3-5 ORACIONES: Que esta mal, por que es un problema, que impacto tiene, y que riesgo introduce. No solo 'falta auth' sino explica el escenario de ataque concreto.", "remediation": "PARRAFO DE 2-4 ORACIONES con pasos concretos y especificos. Ejemplo: 'Implementar un middleware de autenticacion JWT en el router principal de Gin usando gin-jwt o una solucion custom. Configurar el middleware para validar tokens en todos los endpoints excepto GET /health y POST /auth/login. Asegurar que los tokens tengan expiracion maxima de 1 hora y que se valide la firma con RS256.'", "evidence_refs": ["<evidence_id>"]}
  ],
  "tests": [
    {"name": "Nombre descriptivo del test", "description": "PARRAFO DE 2-3 ORACIONES: Que verifica este test, POR QUE es importante verificarlo, y que impacto tendria si falla. No solo 'test de auth' sino explica el escenario.", "flow": "Que flujo de negocio cubre", "importance": "critical|high|medium|low", "request": {"method": "GET", "path": "/x"}, "expected": {"status": 200, "description": "Descripcion clara de como debe verse la respuesta correcta y por que"}, "evidence_refs": ["<evidence_id>"]}
  ],
  "open_questions": [
    {"question": "Pregunta ESPECIFICA que un auditor haria. No 'hay auth?' sino 'Se observo que todos los endpoints responden sin autenticacion — es intencionalmente un servicio interno sin auth, o hay un middleware desactivado? Si es intencional, como se protege el acceso a nivel de red?'", "why_missing": "Explicacion de que evidencia faltaria para responder"}
  ],
  "guided_tour": [],
  "scenario_suggestions": []
}

IDIOMA
1) TODO el texto de los valores en ESPANOL. Campos JSON en ingles.

FUENTES PERMITIDAS
2) Solo informacion del DOSSIER. NUNCA inventes datos que no esten en la evidencia.

CALIDAD DE ESCRITURA — ESTO ES LO MAS IMPORTANTE
3) Escribi como un CONSULTOR EXPERTO que presenta un informe a un equipo tecnico, no como una maquina que llena campos.
   - Cada campo de texto debe ser un PARRAFO completo, no una oracion telegrafada.
   - Usa datos concretos: numeros, endpoints especificos, status codes observados, contenido de responses.
   - Explica el POR QUE y el IMPACTO, no solo el QUE.
   - MALO: "Falta autenticacion en varios endpoints."
   - BUENO: "Se observo que 45 de los 62 endpoints descubiertos responden exitosamente sin ningun tipo de credencial. Esto incluye operaciones criticas como POST /api/v1/orders (creacion de ordenes), DELETE /api/v1/users/{id} (eliminacion de usuarios), y PUT /api/v1/config (modificacion de configuracion). Un atacante con acceso a la red podria crear datos falsos, eliminar informacion legitima, o modificar la configuracion del servicio sin dejar rastro de identidad."

REGLAS DE TRAZABILIDAD
4) SIEMPRE incluir 'evidence_refs' con IDs reales de dossier.evidence_samples. Distribuir: usar 10-20 evidence_ids distintos.

REGLAS DE CONTENIDO
5) security_assessment.summary: MINIMO 5 oraciones. Debe ser un parrafo ejecutivo completo.
6) Cada campo de behavior_assessment: MINIMO 3 oraciones con ejemplos concretos.
7) facts: 10-20 hechos. Cada uno es una ORACION COMPLETA con datos especificos.
8) inferences: 8-15 inferencias. Cada una explica el razonamiento, no solo la conclusion.
9) improvements: 8-15 mejoras. Cada description y remediation son PARRAFOS, no oraciones sueltas.
10) tests: 8-15 tests. Cada description explica POR QUE importa.
11) open_questions: 5-15 preguntas ESPECIFICAS de auditor.
12) No contradigas la evidencia. Anomalias = "anomalia observada", no bug confirmado.
13) 'overview', 'data_models', 'flows': dejar null/vacios.

FORMATO FINAL
14) Solo JSON valido. Sin backticks, sin markdown, sin texto fuera del JSON.

DOSSIER:
%s`
