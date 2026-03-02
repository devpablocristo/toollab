# REVIEW REQUEST: ToolLab Documentation Generation Pipeline

Necesito que revises cómo ToolLab genera documentación de APIs automáticamente.
ToolLab analiza un proyecto (código fuente + runtime HTTP) y envía los resultados a un LLM para generar documentación.

Revisá:
1. Si el PROMPT está bien diseñado para el objetivo
2. Si el DOSSIER (datos enviados) tiene la información correcta y está estructurado eficientemente
3. Si hay información que falta o sobra
4. Si el OUTPUT generado es bueno, y qué se podría mejorar
5. Qué cambiarias del prompt, del dossier, o de ambos

## CONTEXTO TÉCNICO

- Modelo: Gemini 2.5 Flash (via Vertex AI)
- Mode: TextPrompt (sin constrained JSON decoding, el LLM genera Markdown puro)
- Temperature: 0.2
- MaxOutputTokens: 65536
- El backend wrappea el Markdown resultante en JSON después de recibirlo
- Reintentos: 3 con backoff (5s, 10s)
- El dossier se concatena al prompt como: prompt + '\n\nDOSSIER:\n' + JSON
- No hay system prompt, no hay multi-turn. Un solo user message.

## TIEMPOS Y COSTOS

- Dossier: 65KB (~17K tokens input)
- Prompt: 4KB (~1K tokens)
- Output generado: 53KB (~13K tokens)
- Tiempo de generación: 1min 24s
- Costo estimado por run: ~$0.010

---
## 1. PROMPT (lo que le dice al LLM qué hacer)
---
```
Eres un escritor tecnico senior. Audiencia: desarrolladores que no conocen esta API.

Recibes un DOSSIER MINI curado con evidencia real obtenida por analisis estatico (AST) y dinamico (HTTP runtime):
- service: identidad (nombre real del proyecto, source_path, framework, base_url, health endpoints)
- domains[]: packages del codigo fuente con sus handlers — esto revela la organizacion interna
- dtos[]: Data Transfer Objects reales del codigo — estos son los modelos de datos
- endpoints[]: catalogo completo, cada uno con handler_symbol, handler_package, handler_file, group_label, auth classification (PROVEN_REQUIRED / PROVEN_NOT_REQUIRED / UNKNOWN), y hasta 2 samples curados (happy_sample con response body completo + error_sample)
- auth_summary: conteos proven/unknown + discrepancias AST vs runtime
- middlewares[]: indice plano (id, name, kind, source)
- findings: resumen (counts por severity/category) + top 3 highlights
- metrics: requests totales, success rate, latencias, coverage

COMO INTERPRETAR EL DOSSIER:
- service.name es el nombre real del proyecto (ej: "nexus-core"), NO un hostname
- domains[] te dice como esta organizado el codigo. Cada package es un dominio funcional
- dtos[] te dice que datos maneja cada dominio. Relacionalos con los endpoints via handler_package
- Los happy_sample con status 200 muestran la respuesta REAL del endpoint (no inventada)
- handler_package + handler_symbol te dicen QUE HACE cada endpoint (ej: package "actions" + handler "h.apply" = aplicar una accion)

MISION: Producir documentacion Markdown completa, precisa y util para un desarrollador que necesita integrar esta API.

REGLAS DURAS (no negociable):
1. NO INVENTES. Si no hay evidencia, escribi "Sin evidencia disponible" o "UNKNOWN".
2. Toda afirmacion tecnica debe citar [evidence_id] entre corchetes cuando haya sample.
3. Si auth es UNKNOWN para un endpoint, NO afirmes que requiere o no requiere auth.
4. Escribi en ESPANOL. Titulos y prosa en espanol.
5. Usa los datos del dossier tal cual. No reinterpretes metricas ni inventes flujos.
6. Inferi el proposito de cada endpoint a partir de: handler_package, handler_symbol, path, DTOs usados en ese package, y response body real. Esto NO es inventar — es interpretar evidencia.

ESTRUCTURA (estos titulos exactos, en este orden):

# {service.name} — Documentacion API

## 1. Resumen
Que es este servicio, para que sirve (inferir de domains + endpoints), framework, base_url, como esta organizado internamente (listar los dominios principales). 5-8 oraciones.

## 2. Quickstart
3-5 comandos curl listos para copiar/pegar, usando evidence real (citar evidence_id).
Incluir: health check, un GET publico, un request protegido sin auth (mostrar el 401).
Si no hay credenciales conocidas, decirlo explicitamente.

## 3. Autenticacion
Mecanismos observados, como autenticar, que falta saber.
Tabla resumida: cuantos endpoints PROVEN_REQUIRED, PROVEN_NOT_REQUIRED, UNKNOWN.
Discrepancias AST vs runtime si existen.

## 4. Modelos de datos
Listar los DTOs agrupados por dominio/package. Para cada DTO: nombre, campos, y en que endpoints se usa (inferir por package compartido).

## 5. Endpoints por dominio
Agrupar endpoints por handler_package (no por URL prefix). Cada grupo = un dominio funcional.
Por cada dominio: 1 parrafo explicando que hace ese dominio (inferir de handlers + DTOs + responses).
Tabla con method, path, auth, statuses_seen, handler_symbol.
Para los endpoints con happy_sample: mostrar ejemplo request/response completo.

## 6. Middlewares
Tabla: id, nombre, tipo (auth/logging/cors/ratelimit/etc), archivo fuente.
Solo si hay middlewares detectados.

## 7. Hallazgos relevantes
Solo findings.highlights (top 3). Titulo, severidad, descripcion breve, evidence_ids.
Counts generales: "Se detectaron N findings (X high, Y medium, Z low)."

## 8. Metricas de calidad
Requests totales, success rate, p50/p95 latency, coverage, endpoints testeados.

## 9. Preguntas abiertas
Lo que falta para completar la doc: credenciales, contratos, endpoints sin evidence, etc.

SALIDA: Markdown puro. No envuelvas en JSON. No uses code fences alrededor del documento.
Escribi el Markdown directamente, empezando con el titulo H1.
```

---
## 2. DOSSIER (los datos que se envían como evidencia)
---

El dossier completo tiene 60 endpoints, 71 DTOs, 20 domains. Muestro la estructura completa:

### 2.1 Service + Stats + Metrics
```json
{
  "schema_version": "docs-mini-v1",
  "run_id": "3883f966-436f-4a88-8d18-f0b92faa12ae",
  "run_mode": "online_strong",
  "service": {
    "name": "nexus-core",
    "source_path": "/home/pablo/Projects/Pablo/nexus/nexus-core",
    "framework": "gin",
    "base_url": "http://host.docker.internal:8080",
    "base_paths": [
      "/healthz"
    ],
    "versioning_hint": "v1",
    "health_endpoints": [
      "/healthz"
    ],
    "content_types": {
      "produces": [
        "text/html; charset=utf-8",
        "application/json; charset=utf-8",
        "text/plain"
      ]
    }
  },
  "metrics": {
    "total_requests": 2080,
    "success_rate": 0.11826923076923077,
    "p50_ms": 0,
    "p95_ms": 4,
    "endpoints_tested": 61,
    "endpoints_total": 60,
    "coverage_pct": 101.66666666666666
  },
  "stats": {
    "endpoints_count": 60,
    "samples_count": 40,
    "middleware_count": 0
  },
  "findings": {
    "total": 8,
    "by_severity": {
      "low": 2,
      "medium": 6
    },
    "by_category": {
      "contract": 2,
      "logic": 3,
      "observability": 2,
      "rate_limit": 1
    },
    "highlights": [
      {
        "title": "Duplicate creation accepted on POST /v1/policy-proposals",
        "severity": "medium",
        "category": "logic",
        "description": "",
        "evidence_refs": [
          "c26fb374e0088b11",
          "5a148aa2162c40fc",
          "f9bc970f719f98f0"
        ]
      },
      {
        "title": "Duplicate creation accepted on POST /mcp",
        "severity": "medium",
        "category": "logic",
        "description": "",
        "evidence_refs": [
          "809a292b03be31dd",
          "6f11763e2c6c437f",
          "aa87f342585d529e"
        ]
      },
      {
        "title": "OpenAPI coverage mismatch against discovered endpoints",
        "severity": "medium",
        "category": "contract",
        "description": "Spec endpoints=11, AST endpoints=60, AST missing in spec=49, spec missing in AST=0, match=18.3%.",
        "evidence_refs": [
          "772fc65224545cc8"
        ]
      }
    ]
  },
  "auth_summary": {
    "mechanisms": null,
    "proven_required": 14,
    "proven_not_required": 6,
    "unknown": 40,
    "discrepancy_count": 47,
    "discrepancy_examples": [
      {
        "endpoint_id": "f1bdec6a7ac78f9c",
        "description": "No auth middleware en AST pero runtime deniega acceso a POST /a2a/call",
        "ast_says": "no_auth_middleware",
        "runtime_says": "denied"
      },
      {
        "endpoint_id": "d424c067612f132d",
        "description": "No auth middleware en AST pero runtime deniega acceso a POST /v1/actions/apply",
        "ast_says": "no_auth_middleware",
        "runtime_says": "denied"
      },
      {
        "endpoint_id": "49d01664bda39a6c",
        "description": "No auth middleware en AST pero runtime deniega acceso a POST /v1/actions/rollback",
        "ast_says": "no_auth_middleware",
        "runtime_says": "denied"
      },
      {
        "endpoint_id": "5f6c96c38ca21589",
        "description": "No auth middleware en AST pero runtime deniega acceso a GET /v1/actions",
        "ast_says": "no_auth_middleware",
        "runtime_says": "denied"
      },
      {
        "endpoint_id": "4d6465ecc395831b",
        "description": "No auth middleware en AST pero runtime deniega acceso a GET /v1/admin/bootstrap",
        "ast_says": "no_auth_middleware",
        "runtime_says": "denied"
      }
    ]
  },
  "middlewares": []
}
```

### 2.2 Domains (20 total)
```json
[
  {
    "package": "nexus-core/cmd/mock-tools",
    "endpoint_count": 5,
    "handlers": [
      "(anonymous)"
    ]
  },
  {
    "package": "nexus-core/internal/a2a",
    "endpoint_count": 1,
    "handlers": [
      "h.call"
    ]
  },
  {
    "package": "nexus-core/internal/actions",
    "endpoint_count": 3,
    "handlers": [
      "h.apply",
      "h.rollback",
      "h.list"
    ]
  },
  {
    "package": "nexus-core/internal/admin",
    "endpoint_count": 4,
    "handlers": [
      "h.bootstrap",
      "h.getTenantSettings",
      "h.upsertTenantSettings",
      "h.listActivity"
    ]
  },
  {
    "package": "nexus-core/internal/agents/executive_qa",
    "endpoint_count": 1,
    "handlers": [
      "h.ask"
    ]
  },
  {
    "package": "nexus-core/internal/assistant",
    "endpoint_count": 1,
    "handlers": [
      "h.query"
    ]
  },
  {
    "package": "nexus-core/internal/audit",
    "endpoint_count": 2,
    "handlers": [
      "h.query",
      "h.export"
    ]
  },
  {
    "package": "nexus-core/internal/egress",
    "endpoint_count": 3,
    "handlers": [
      "h.upsert",
      "h.list",
      "h.delete"
    ]
  },
  {
    "package": "nexus-core/internal/events",
    "endpoint_count": 1,
    "handlers": [
      "h.list"
    ]
  },
  {
    "package": "nexus-core/internal/gateway",
    "endpoint_count": 2,
    "handlers": [
      "h.run",
      "h.simulate"
    ]
  },
  {
    "package": "nexus-core/internal/identity",
    "endpoint_count": 3,
    "handlers": [
      "h.configStatus",
      "h.authorize",
      "h.callback"
    ]
  },
  {
    "package": "nexus-core/internal/incidents",
    "endpoint_count": 4,
    "handlers": [
      "h.create",
      "h.list",
      "h.get",
      "h.close"
    ]
  },
  {
    "package": "nexus-core/internal/mcp",
    "endpoint_count": 1,
    "handlers": [
      "h.rpc"
    ]
  },
  {
    "package": "nexus-core/internal/ops/actionengine",
    "endpoint_count": 3,
    "handlers": [
      "h.dryRun",
      "h.apply",
      "h.rollback"
    ]
  },
  {
    "package": "nexus-core/internal/policy",
    "endpoint_count": 3,
    "handlers": [
      "h.createForTool",
      "h.listForTool",
      "h.updateByID"
    ]
  },
  {
    "package": "nexus-core/internal/policyproposal",
    "endpoint_count": 5,
    "handlers": [
      "h.create",
      "h.list",
      "h.approve",
      "h.reject",
      "h.shadow"
    ]
  },
  {
    "package": "nexus-core/internal/secrets",
    "endpoint_count": 3,
    "handlers": [
      "h.upsert",
      "h.list",
      "h.delete"
    ]
  },
  {
    "package": "nexus-core/internal/tool",
    "endpoint_count": 4,
    "handlers": [
      "h.create",
      "h.list",
      "h.get",
      "h.update"
    ]
  },
  {
    "package": "nexus-core/internal/world",
    "endpoint_count": 6,
    "handlers": [
      "h.listRuns",
      "h.state",
      "h.events",
      "h.eventsStream",
      "h.createRun",
      "h.replay"
    ]
  },
  {
    "package": "nexus-core/wire",
    "endpoint_count": 5,
    "handlers": [
      "(anonymous)"
    ]
  }
]
```

### 2.3 DTOs (primeros 10 de 71)
```json
[
  {
    "name": "APIError",
    "package": "nexus-core/pkg/types",
    "file": "pkg/types/http_errors.go",
    "fields": [
      "Code",
      "Message"
    ]
  },
  {
    "name": "ActionEngineRequest",
    "package": "nexus-core/internal/ops/actionengine/handler/dto",
    "file": "internal/ops/actionengine/handler/dto/dto.go",
    "fields": [
      "IncidentID",
      "ProposalID",
      "ActionType",
      "Scope",
      "TTLSeconds",
      "Params",
      "EvidenceRefs",
      "ApprovalGranted",
      "ApprovalComment"
    ]
  },
  {
    "name": "ActionEngineResponse",
    "package": "nexus-core/internal/ops/actionengine/handler/dto",
    "file": "internal/ops/actionengine/handler/dto/dto.go",
    "fields": [
      "RequestID",
      "ProposalID",
      "ExecutionID",
      "Status",
      "ActionType",
      "IdempotencyKey",
      "ScopeHash",
      "ParamsHash",
      "ApprovalRequired",
      "Replay",
      "Scope",
      "Params"
    ]
  },
  {
    "name": "ActionHint",
    "package": "nexus-core/internal/assistant/handler/dto",
    "file": "internal/assistant/handler/dto/dto.go",
    "fields": [
      "Label",
      "ActionType",
      "Payload"
    ]
  },
  {
    "name": "ActionItem",
    "package": "nexus-core/internal/actions/handler/dto",
    "file": "internal/actions/handler/dto/dto.go",
    "fields": [
      "ID",
      "ScopeType",
      "ScopeID",
      "ActionType",
      "Params",
      "TTLSeconds",
      "Status",
      "EvidenceRefs",
      "CreatedBy",
      "CreatedAt",
      "RolledBackAt",
      "RolledBackBy"
    ]
  },
  {
    "name": "Actor",
    "package": "nexus-core/internal/ops/eventstore/usecases/domain",
    "file": "internal/ops/eventstore/usecases/domain/entities.go",
    "fields": [
      "ActorID",
      "ActorType"
    ]
  },
  {
    "name": "AdminActivityItem",
    "package": "nexus-core/internal/admin/handler/dto",
    "file": "internal/admin/handler/dto/dto.go",
    "fields": [
      "ID",
      "Actor",
      "Action",
      "ResourceType",
      "ResourceID",
      "Payload",
      "CreatedAt"
    ]
  },
  {
    "name": "AdminActivityResponse",
    "package": "nexus-core/internal/admin/handler/dto",
    "file": "internal/admin/handler/dto/dto.go",
    "fields": [
      "Items"
    ]
  },
  {
    "name": "ApplyActionRequest",
    "package": "nexus-core/internal/actions/handler/dto",
    "file": "internal/actions/handler/dto/dto.go",
    "fields": [
      "ScopeType",
      "ScopeID",
      "ActionType",
      "Params",
      "TTLSeconds",
      "EvidenceRefs"
    ]
  },
  {
    "name": "ApplyRequest",
    "package": "nexus-core/internal/actions",
    "file": "internal/actions/usecases.go",
    "fields": [
      "ScopeType",
      "ScopeID",
      "ActionType",
      "Params",
      "TTLSeconds",
      "EvidenceRefs"
    ]
  }
]
```

### 2.4 Endpoints CON happy_sample (ejemplo: 2 de 20)
```json
[
  {
    "endpoint_id": "5f6c96c38ca21589",
    "method": "GET",
    "path": "/v1/actions",
    "handler_symbol": "h.list",
    "handler_file": "internal/actions/handler.go",
    "handler_line": 29,
    "handler_package": "nexus-core/internal/actions",
    "auth": "PROVEN_REQUIRED",
    "happy_sample": {
      "evidence_id": "21b8523ece2daf80",
      "method": "GET",
      "path": "/v1/actions",
      "req_headers": {
        "Accept": "application/json",
        "X-Nexus-Core-Key": "nexus-co****"
      },
      "status": 200,
      "resp_snippet": "{\"items\":[{\"id\":\"b5870769-78f6-4a81-a3e7-5e6743bd296f\",\"scope_type\":\"tenant\",\"action_type\":\"throttle_tenant_rpm\",\"params\":{\"per_minute\":10},\"ttl_seconds\":300,\"status\":\"active\",\"evidence_refs\":[\"event:3061\",\"event:3062\",\"event:3063\",\"event:3064\",\"event:3065\",\"event:3066\",\"event:3067\",\"event:3068\",\"event:3069\",\"event:3070\",\"event:3071\",\"event:3072\",\"event:3073\",\"event:3074\",\"event:3075\",\"event:3076\",\"event:3077\",\"event:3078\",\"event:3079\",\"event:3080\"],\"created_by\":\"operator/responder\",\"created_at\":\"2026-03-02T15:17:57Z\"},{\"id\":\"7a8565ed-3460-4917-b12e-bdb6c5bd36bf\",\"scope_type\":\"tenant\",\"action_type\":\"throttle_tenant_rpm\",\"params\":{\"per_minute\":10},\"ttl_seconds\":300,\"status\":\"expired\",\"evidence_refs\":[\"event:3061\",\"event:3062\",\"event:3063\",\"event:3064\",\"event:3065\",\"event:3066\",\"event:3067\",\"event:3068\",\"event:3069\",\"event:3070\",\"event:3071\",\"event:3072\",\"event:3073\",\"event:3074\",\"event:3075\",\"event:3076\",\"event:3077\",\"event:3078\",\"event:3079\",\"event:3080\"],\"created_by\":\"operator/responder\",\"created_at\":\"2026-03-01T05:28:16Z\"},{\"id\":\"01e44600-bddb-4949-a219-05212886f5e3\",\"scope_type\":\"tenant\",\"action_type\":\"throttle_tenant_rpm\",\"params\":{\"per_minute\":10},\"ttl_seconds\":300,\"status\":\"expired\",\"evidence_refs\":[\"event:3061\",\"event:3062\",\"event:3063\",\"event:3064\",\"event:3065\",\"event:3066\",\"event:3067\",\"event:3068\",\"event:3069\",\"event:3070\",\"event:3071\",\"event:3072\",\"event:3073\",\"event:3074\",\"event:3075\",\"event:3076\",\"event:3077\",\"event:3078\",\"event:3079\",\"event:3080\"],\"created_by\":\"",
      "latency_ms": 0
    },
    "error_sample": {
      "evidence_id": "1f025fe398f7747a",
      "method": "GET",
      "path": "/v1/actions",
      "req_headers": {
        "Accept": "application/json",
        "Authorization": "Bearer i****"
      },
      "status": 401,
      "resp_snippet": "{\"request_id\":\"6b3a19be-553f-40a6-9702-a8d67a5d02af\",\"error\":{\"code\":\"UNAUTHORIZED\",\"message\":\"missing api key\"}}",
      "latency_ms": 0
    },
    "statuses_seen": [
      200,
      401,
      404
    ]
  },
  {
    "endpoint_id": "4d6465ecc395831b",
    "method": "GET",
    "path": "/v1/admin/bootstrap",
    "handler_symbol": "h.bootstrap",
    "handler_file": "internal/admin/handler.go",
    "handler_line": 27,
    "handler_package": "nexus-core/internal/admin",
    "auth": "PROVEN_REQUIRED",
    "happy_sample": {
      "evidence_id": "5b6a7ebd924379fb",
      "method": "GET",
      "path": "/v1/admin/bootstrap",
      "req_headers": {
        "Content-Type": "application/json",
        "X-Nexus-Core-Key": "nexus-co****"
      },
      "status": 200,
      "resp_snippet": "{\"org_id\":\"06974534-2ba8-4d0b-829a-a9594242e2a9\",\"scopes\":[\"tools:read\",\"tools:write\",\"policy:read\",\"policy:write\",\"egress:read\",\"egress:write\",\"audit:read\",\"gateway:run\",\"gateway:simulate\",\"mcp:read\",\"mcp:call\",\"a2a:call\",\"admin:secrets\",\"admin:console:read\",\"admin:console:write\"],\"auth_method\":\"api_key\",\"can_read_admin\":true,\"can_write_admin\":true,\"tenant_settings\":{\"plan_code\":\"enterprise\",\"hard_limits\":{\"audit_retention_days\":365,\"run_rpm\":5000,\"tools_max\":250},\"updated_by\":\"sim-engine-seed\",\"updated_at\":\"2026-02-27T16:06:29Z\",\"created_at\":\"2026-02-27T16:06:29Z\"}}",
      "latency_ms": 0
    },
    "error_sample": {
      "evidence_id": "16ed9d0a85edd07e",
      "method": "GET",
      "path": "/v1/admin/bootstrap",
      "req_headers": {
        "Accept": "application/json",
        "Authorization": "Bearer i****"
      },
      "status": 401,
      "resp_snippet": "{\"request_id\":\"f9497c98-ee12-43ea-89ae-a6a88e976385\",\"error\":{\"code\":\"UNAUTHORIZED\",\"message\":\"missing api key\"}}",
      "latency_ms": 0
    },
    "statuses_seen": [
      200,
      401,
      404
    ]
  }
]
```

### 2.5 Endpoints SIN happy_sample (ejemplo: 2 de 40)
```json
[
  {
    "endpoint_id": "f1bdec6a7ac78f9c",
    "method": "POST",
    "path": "/a2a/call",
    "handler_symbol": "h.call",
    "handler_file": "internal/a2a/handler.go",
    "handler_line": 27,
    "handler_package": "nexus-core/internal/a2a",
    "auth": "UNKNOWN",
    "statuses_seen": [
      400,
      401,
      404
    ]
  },
  {
    "endpoint_id": "d424c067612f132d",
    "method": "POST",
    "path": "/v1/actions/apply",
    "handler_symbol": "h.apply",
    "handler_file": "internal/actions/handler.go",
    "handler_line": 27,
    "handler_package": "nexus-core/internal/actions",
    "auth": "UNKNOWN",
    "statuses_seen": [
      400,
      401,
      404
    ]
  }
]
```

### 2.6 Distribución de endpoints
- Total: 60
- Con happy_sample: 20
- Sin happy_sample: 40
- Con error_sample: 20
- Sin ningún sample: 40

---
## 3. OUTPUT GENERADO (primeros 8000 chars del Markdown)
---
```markdown
# nexus-core — Documentacion API

## 1. Resumen
El servicio `nexus-core` es una API gateway multi-inquilino para la ejecución controlada de herramientas, gestión de políticas, auditoría, manejo de incidentes y funcionalidades de identidad. Construido con el framework Gin, su propósito principal es orquestar y asegurar las interacciones con diversas herramientas y sistemas internos. La API está organizada en dominios funcionales clave como acciones, administración, agentes, asistente, auditoría, egreso, eventos, gateway, identidad, incidentes, MCP, políticas, propuestas de políticas, secretos, herramientas y el "mundo" de ejecución. La URL base para interactuar con este servicio es `http://host.docker.internal:8080`.

## 2. Quickstart
Aquí tienes algunos comandos `curl` para empezar a interactuar con la API. Ten en cuenta que para los endpoints protegidos, se requiere una clave API (`X-Nexus-Core-Key`). Las credenciales específicas no están disponibles en la evidencia proporcionada.

1.  **Verificar el estado de salud del servicio:**
    ```bash
    curl -X GET "http://host.docker.internal:8080/healthz" -H "Accept: application/json"
    # Respuesta esperada: {"ok":true} [03baf3663e49ddd9]
    ```

2.  **Acceder a la documentación OpenAPI (pública):**
    ```bash
    curl -X GET "http://host.docker.internal:8080/openapi.yaml" -H "Accept: application/json"
    # Respuesta esperada: Contenido YAML de la especificación OpenAPI [3031139b673a6dd8]
    ```

3.  **Intentar acceder a un endpoint protegido sin autenticación (resultará en 401 Unauthorized):**
    ```bash
    curl -X GET "http://host.docker.internal:8080/v1/actions" -H "Accept: application/json"
    # Respuesta esperada: {"request_id":"6b3a19be-553f-40a6-9702-a8d67a5d02af","error":{"code":"UNAUTHORIZED","message":"missing api key"}} [1f025fe398f7747a]
    ```

## 3. Autenticacion
La evidencia sugiere que la autenticación se realiza principalmente a través de una clave API en el encabezado `X-Nexus-Core-Key`, y también se menciona un modo `Bearer` para JWT en la especificación OpenAPI. No se observaron mecanismos de autenticación específicos más allá de estos encabezados. Las credenciales exactas para `X-Nexus-Core-Key` o tokens `Bearer` no están disponibles en la evidencia.

**Resumen de Autenticación de Endpoints:**

| Clasificación de Autenticación | Cantidad de Endpoints |
| :----------------------------- | :-------------------- |
| PROVEN_REQUIRED                | 14                    |
| PROVEN_NOT_REQUIRED            | 6                     |
| UNKNOWN                        | 40                    |

**Discrepancias AST vs. Runtime:**
Se detectaron 47 discrepancias entre el análisis estático (AST) y el comportamiento en tiempo de ejecución (runtime) con respecto a la autenticación. Esto indica que muchos endpoints que el AST no identificó con middlewares de autenticación, en runtime denegaron el acceso, sugiriendo que la autenticación se aplica de alguna manera que no es visible directamente en el AST o que hay una capa de seguridad externa.

Ejemplos de discrepancias:
*   `POST /a2a/call`: AST indica `no_auth_middleware`, pero runtime deniega el acceso [f1bdec6a7ac78f9c].
*   `POST /v1/actions/apply`: AST indica `no_auth_middleware`, pero runtime deniega el acceso [d424c067612f132d].
*   `POST /v1/actions/rollback`: AST indica `no_auth_middleware`, pero runtime deniega el acceso [49d01664bda39a6c].
*   `GET /v1/actions`: AST indica `no_auth_middleware`, pero runtime deniega el acceso [5f6c96c38ca21589].
*   `GET /v1/admin/bootstrap`: AST indica `no_auth_middleware`, pero runtime deniega el acceso [4d6465ecc395831b].

## 4. Modelos de datos
Los siguientes son los Data Transfer Objects (DTOs) reales observados en el código, agrupados por el paquete al que pertenecen. Estos DTOs definen la estructura de los datos que la API consume y produce.

**Paquete: `nexus-core/pkg/types`**
*   **APIError**: Representa un error genérico de la API.
    *   Campos: `Code`, `Message`
*   **ErrorResponse**: Estructura para respuestas de error de la API.
    *   Campos: `RequestID`, `Error`

**Paquete: `nexus-core/internal/ops/actionengine/handler/dto`**
*   **ActionEngineRequest**: Solicitud para el motor de acciones.
    *   Campos: `IncidentID`, `ProposalID`, `ActionType`, `Scope`, `TTLSeconds`, `Params`, `EvidenceRefs`, `ApprovalGranted`, `ApprovalComment`
*   **ActionEngineResponse**: Respuesta del motor de acciones.
    *   Campos: `RequestID`, `ProposalID`, `ExecutionID`, `Status`, `ActionType`, `IdempotencyKey`, `ScopeHash`, `ParamsHash`, `ApprovalRequired`, `Replay`, `Scope`, `Params`

**Paquete: `nexus-core/internal/assistant/handler/dto`**
*   **ActionHint**: Sugerencia de acción del asistente.
    *   Campos: `Label`, `ActionType`, `Payload`
*   **AskRequest**: Solicitud para el asistente ejecutivo de QA.
    *   Campos: `Question`, `IncidentID`
*   **AskResponse**: Respuesta del asistente ejecutivo de QA.
    *   Campos: `Answer`, `EvidenceRefs`, `ProposedActionID`, `ProposedActionType`
*   **QueryRequest**: Solicitud de consulta al asistente.
    *   Campos: `Query`
*   **QueryResponse**: Respuesta de consulta del asistente.
    *   Campos: `Summary`, `Tables`, `Actions`
*   **TablePayload**: Estructura para datos tabulares en la respuesta del asistente.
    *   Campos: `Title`, `Columns`, `Rows`

**Paquete: `nexus-core/internal/actions/handler/dto`**
*   **ActionItem**: Representa una acción individual.
    *   Campos: `ID`, `ScopeType`, `ScopeID`, `ActionType`, `Params`, `TTLSeconds`, `Status`, `EvidenceRefs`, `CreatedBy`, `CreatedAt`, `RolledBackAt`, `RolledBackBy`
*   **ApplyActionRequest**: Solicitud para aplicar una acción.
    *   Campos: `ScopeType`, `ScopeID`, `ActionType`, `Params`, `TTLSeconds`, `EvidenceRefs`
*   **ListActionsResponse**: Respuesta para listar acciones.
    *   Campos: `Items`
*   **RollbackActionRequest**: Solicitud para revertir una acción.
    *   Campos: `ActionID`

**Paquete: `nexus-core/internal/ops/eventstore/usecases/domain`**
*   **Actor**: Representa un actor en el sistema.
    *   Campos: `ActorID`, `ActorType`
*   **Correlation**: Información de correlación de eventos.
    *   Campos: `RequestID`, `IncidentID`, `ActionID`
*   **Envelope**: Envoltorio para eventos.
    *   Campos: `ID`, `EventType`, `Version`, `OccurredAt`, `OrgID`, `Correlation`, `Actor`, `Source`, `Payload`

**Paquete: `nexus-core/internal/admin/handler/dto`**
*   **AdminActivityItem**: Un elemento de actividad administrativa.
    *   Campos: `ID`, `Actor`, `Action`, `ResourceType`, `ResourceID`, `Payload`, `CreatedAt`
*   **AdminActivityResponse**: Respuesta para listar actividades administrativas.
    *   Campos: `Items`
*   **BootstrapResponse**: Respuesta de inicialización de administración.
    *   Campos: `OrgID`, `Actor`, `Role`, `Scopes`, `AuthMethod`, `CanReadAdmin`, `CanWriteAdmin`, `TenantSetting`
*   **TenantSettings**: Configuración de un inquilino.
    *   Campos: `PlanCode`, `HardLimits`, `UpdatedBy`, `UpdatedAt`, `CreatedAt`
*   **UpsertTenantSettingsRequest**: Solicitud para crear o actualizar la configuración de un inquilino.
    *   Campos: `PlanCode`, `HardLimits`

**Paquete: `nexus-core/internal/a2a/handler/dto`**
*   **CallRequest**: Solicitud para una llamada A2A (Agent-to-Agent o Application-to-Application).
    *   Campos: `RequestID`, `ToolName`, `Input`, `Context`, `TimeoutMS`, `IdempotencyKey`
*   **CallResponse**: Respuesta de una llamada A2A.
    *   Campos: `RequestID`, `Decision`, `ToolName`, `Status`, `Reason`, `Result`, `Error`, `LatencyMS`, `Idempotency`
*   **ErrorObj**: Objeto de error para respuestas A2A.
    *   Campos: `Code`, `Message`
*   **Idempotency**: Información de idempotencia.
    *   Campos: `Present`, `Outcome`

**Paquete: `nexus-core/internal/audit/handler/dto`**
*   **AuditItem**: Un elemento de auditoría.
    *   Campos: `RequestID`, `OrgID`, `ToolName`, `Actor`, `Role`, `Scopes`, `Decision`, `Status`, `Reason`, `LatencyMS`, `Idemp
```

---
## 4. PREGUNTAS ESPECÍFICAS
---

1. El prompt es eficiente? Hay instrucciones redundantes o que confunden al modelo?
2. La estructura del dossier es óptima para que el LLM genere buena documentación? Qué sobra, qué falta?
3. 40 endpoints sin happy_sample (solo metadata): ayudan o son ruido?
4. 71 DTOs con solo nombre+campos (sin tipos, sin descripciones): es suficiente?
5. El output generado es bueno para un desarrollador que necesita integrar la API?
6. Hay algún anti-pattern en cómo armamos el prompt+datos?
7. Cómo mejorarías el ratio calidad/tokens?