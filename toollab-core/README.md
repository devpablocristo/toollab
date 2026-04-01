# toollab-core

Backend Go de ToolLab. Orquesta `targets`, `runs`, pipeline de análisis, artifacts, exports, `IntelligenceService` determinístico y `SynthesisService` LLM bounded.

## Requisitos

- Go 1.25+
- CGO habilitado para SQLite

## Quickstart

```bash
mkdir -p data
CGO_ENABLED=1 go run ./cmd/toollab-dashboard
```

Servidor local: `http://localhost:8090`

## Variables de entorno

| Variable | Default | Descripción |
|---|---|---|
| `TOOLLAB_ADDR` | `:8090` | Dirección HTTP del backend |
| `TOOLLAB_DB_PATH` | `./data/toollab.db` | SQLite del laboratorio |
| `TOOLLAB_DATA_DIR` | `./data` | Directorio base de artifacts |
| `TOOLLAB_HOST_REWRITE` | vacío | Reescritura host interno, útil con Docker |
| `GOOGLE_PROJECT_ID` / `GOOGLE_CLOUD_PROJECT` | `toollab` | Config de Vertex |
| `GOOGLE_REGION` | `us-central1` | Región Vertex |
| `VERTEX_MODEL` / `GOOGLE_LLM_MODEL` | `gemini-2.5-flash` | Modelo LLM |

## API principal

### Targets

```bash
curl -X POST http://localhost:8090/api/v1/targets \
  -H 'Content-Type: application/json' \
  -d '{
    "name":"my-api",
    "source":{"type":"path","value":"/home/pablo/Projects/my-api"},
    "runtime_hint":{"base_url":"http://localhost:3000"}
  }'

curl http://localhost:8090/api/v1/targets
curl http://localhost:8090/api/v1/targets/{target_id}
curl http://localhost:8090/api/v1/targets/{target_id}/latest-run
```

### Analyze

El loop completo se dispara hoy por SSE desde:

```bash
curl -N -X POST "http://localhost:8090/api/v1/targets/{target_id}/analyze?lang=es" \
  -H 'Accept: text/event-stream'
```

Pipeline real:

`preflight -> astdiscovery -> schema -> smoke -> authmatrix -> fuzz -> logic -> abuse -> confirm -> report`

## Artifacts y endpoints de consulta

```bash
curl http://localhost:8090/api/v1/runs/{run_id}
curl http://localhost:8090/api/v1/runs/{run_id}/docs
curl http://localhost:8090/api/v1/runs/{run_id}/audit
curl http://localhost:8090/api/v1/runs/{run_id}/artifact/run_summary
curl http://localhost:8090/api/v1/runs/{run_id}/artifact/dossier_full
curl http://localhost:8090/api/v1/runs/{run_id}/endpoints
curl http://localhost:8090/api/v1/runs/{run_id}/endpoints/{endpoint_id}
curl http://localhost:8090/api/v1/runs/{run_id}/endpoints/{endpoint_id}/scripts
```

Artifacts relevantes:

- `target_profile`
- `endpoint_catalog`, `router_graph`, `ast_entities`, `ast_code_patterns`
- `schema_registry`, `inferred_contracts`, `semantic_annotations`
- `smoke_results`, `auth_matrix`, `fuzz_results`, `logic_results`, `abuse_results`
- `confirmations`, `findings_raw`, `error_signatures`, `raw_evidence`
- `run_summary`, `scoring`, `dossier_full`, `dossier_docs_mini`, `dossier_llm`
- `endpoint_intelligence`, `endpoint_intelligence_index`, `endpoint_queries`
- `postman_collection`, `curl_book`, `openapi_inferred`, `openapi_ast`
- `llm_docs` y `llm_audit`

## Categorías canónicas del ecosistema

- `IntelligenceService`: `internal/intelligence`, `endpoint_intelligence`, `endpoint_queries` y derivaciones determinísticas post-run
- `SynthesisService`: `internal/llm`, `llm_docs` y `llm_audit` como artefactos batch sobre evidencia ya consolidada

ToolLab no modela un agente conversacional del producto.

## Runtime LLM

- `llm_docs` se genera en background cuando hay artifacts compactados suficientes
- el prompt de auditoría LLM está definido y versionado
- la generación efectiva de `llm_audit` permanece deshabilitada por default en el runtime actual

## Playground

Superficie manual por run:

```bash
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/playground/send \
  -H 'Content-Type: application/json' \
  -d '{"method":"GET","url":"http://localhost:3000/health","endpoint_id":"manual"}'
```

Capacidades:

- `send`
- `replay`
- `auth-profiles`

Guardrails:

- SSRF restringido al host permitido del run
- timeout máximo de 30s
- límite de body de 1MB
- evidencia manual persistida como artifact reutilizable

## Estructura

```text
toollab-core/
├── cmd/toollab-dashboard/
├── internal/
│   ├── target/
│   ├── run/
│   ├── artifact/
│   ├── pipeline/
│   ├── preflight/
│   ├── astdiscovery/
│   ├── schema/
│   ├── smoke/
│   ├── authmatrix/
│   ├── fuzz/
│   ├── logic/
│   ├── abuse/
│   ├── confirm/
│   ├── report/
│   ├── intelligence/
│   ├── exports/
│   ├── llm/
│   ├── playground/
│   └── shared/
├── migrations/
└── data/
```

## Desarrollo

```bash
go test ./...
go build ./cmd/toollab-dashboard
```
