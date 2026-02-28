# toollab-core

Backend del laboratorio ToolLab. Gestiona targets, runs, artifacts, discovery AST, scenario plans, ejecución HTTP y captura de evidencia.

## Requisitos

- Go 1.22+
- CGO habilitado (para SQLite)

## Uso rápido

```bash
make run     # levanta en :8090
make test    # corre todos los tests
make build   # genera bin/toollab-dashboard
```

## Variables de entorno

| Variable | Default | Descripción |
|---|---|---|
| `TOOLLAB_ADDR` | `:8090` | Dirección del servidor |
| `TOOLLAB_DB_PATH` | `./data/toollab.db` | Ruta al archivo SQLite |
| `TOOLLAB_DATA_DIR` | `./data` | Directorio base para artifacts |

## API

### Targets

```bash
# Crear target (source.value = path al repo, base_url para ejecutar)
curl -X POST http://localhost:8090/api/v1/targets \
  -H 'Content-Type: application/json' \
  -d '{"name":"my-api","source":{"type":"path","value":"/workspace/api"},"runtime_hint":{"base_url":"http://localhost:3000"}}'

# Listar targets
curl http://localhost:8090/api/v1/targets

# Obtener target
curl http://localhost:8090/api/v1/targets/{target_id}
```

### Runs

```bash
# Crear run
curl -X POST http://localhost:8090/api/v1/targets/{target_id}/runs \
  -H 'Content-Type: application/json' \
  -d '{"seed":"manual","notes":"primera auditoría"}'

# Listar runs de un target
curl http://localhost:8090/api/v1/targets/{target_id}/runs

# Obtener run
curl http://localhost:8090/api/v1/runs/{run_id}
```

### Discovery (Etapa 3)

```bash
# Descubrir endpoints del código fuente (Go + Chi AST analysis)
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/discover \
  -H 'Content-Type: application/json' \
  -d '{"framework_hint": "chi", "generate_scenario_plan": true}'
```

Respuesta:
```json
{
  "run_id": "...",
  "service_model_revision": 1,
  "model_report_revision": 1,
  "scenario_plan_revision": 1,
  "endpoints_count": 9,
  "confidence": 0.9,
  "gaps": []
}
```

Artifacts generados:
- `service_model` — endpoints detectados con method, path, handler_name, ref (file + line)
- `model_report` — endpoints_count, confidence, gaps (handlers anónimos, routing dinámico, etc.)
- `scenario_plan` — (opcional) plan base auto-generado con un case por endpoint

```bash
# Ver el service model generado
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/service_model

# Ver el model report
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/model_report
```

### Audit (Etapa 4)

```bash
# Ejecutar auditoría determinista sobre los artifacts del run
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/audit \
  -H 'Content-Type: application/json' \
  -d '{}'

# Ejecutar solo reglas específicas
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/audit \
  -H 'Content-Type: application/json' \
  -d '{"rules": ["AUTH_MISSING_2XX", "ERROR_LEAK_STACKTRACE"], "max_findings": 100}'
```

Respuesta:
```json
{
  "run_id": "...",
  "audit_report_revision": 1,
  "findings_total": 11,
  "by_severity": {"high": 9, "medium": 2},
  "by_category": {"auth": 10, "error_leak": 1}
}
```

```bash
# Ver audit report completo
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/audit_report
```

Reglas v1:
| Rule ID | Categoría | Qué detecta |
|---|---|---|
| `AUTH_MISSING_2XX` | auth | Endpoint accesible sin autenticación (2xx sin Authorization/Cookie/X-Api-Key) |
| `ERROR_LEAK_STACKTRACE` | error_leak | Respuesta de error expone detalles internos (panic, goroutine, paths, SQL) |
| `NON_DETERMINISM_REPLAY` | stability | Mismo endpoint devuelve diferentes status codes |
| `CONTRACT_ANOMALY_BASIC` | contract_anomaly | Anomalías de contrato HTTP (body vacío en write, Content-Type ausente) |

### Interpretation LLM (Etapa 5)

```bash
# Ejecutar interpretación LLM bounded (mock provider por default)
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/interpret \
  -H 'Content-Type: application/json' \
  -d '{}'

# Con opciones: modo strict, limitar curación
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/interpret \
  -H 'Content-Type: application/json' \
  -d '{"mode": "lenient", "top_endpoints": 10, "top_findings": 10, "max_snippet_bytes": 2048}'
```

Respuesta:
```json
{
  "run_id": "...",
  "llm_interpretation_revision": 1,
  "facts_count": 2,
  "inferences_count": 1,
  "questions_count": 1,
  "rejected_claims_count": 0,
  "provider_name": "mock",
  "validation_mode": "lenient"
}
```

```bash
# Ver interpretación completa (alias al artifact latest)
curl http://localhost:8090/api/v1/runs/{run_id}/interpretation

# O vía artifacts genérico
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/llm_interpretation
```

Validación:
- **strict**: si algún fact/inference/guided_step/scenario_suggestion no tiene refs válidas → error 400, no se guarda artifact
- **lenient** (default): claims sin refs válidas se degradan a `open_questions` con `why_missing=missing_or_invalid_refs`

Output tipado: `facts[]` (observados), `inferences[]` (con rule_of_inference), `open_questions[]` (con suggested_probe), `guided_tour[]`, `scenario_suggestions[]`

### Scenario Plan (Etapa 2)

```bash
# Subir scenario plan manual (append-only, crea nueva revision)
curl -X PUT http://localhost:8090/api/v1/runs/{run_id}/scenario-plan \
  -H 'Content-Type: application/json' \
  -d '{
    "plan_id": "plan-1",
    "run_id": "{run_id}",
    "schema_version": "v1",
    "cases": [
      {
        "case_id": "case-1",
        "name": "GET /health",
        "enabled": true,
        "tags": ["smoke"],
        "request": {"method": "GET", "path": "/health"}
      }
    ]
  }'

# Obtener scenario plan (latest)
curl http://localhost:8090/api/v1/runs/{run_id}/scenario-plan
```

### Ejecutar Run (Etapa 2)

```bash
# Ejecutar todos los cases del scenario plan contra el target
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/execute \
  -H 'Content-Type: application/json' \
  -d '{}'

# Ejecutar solo ciertos cases o tags
curl -X POST http://localhost:8090/api/v1/runs/{run_id}/execute \
  -H 'Content-Type: application/json' \
  -d '{"subset_case_ids": ["case-1"], "tags": ["smoke"], "timeout_ms": 8000}'
```

### Evidence (Etapa 2)

```bash
# Obtener evidence pack (latest)
curl http://localhost:8090/api/v1/runs/{run_id}/evidence

# Obtener detalle de un evidence item (con bodies completos)
curl http://localhost:8090/api/v1/runs/{run_id}/evidence/items/{evidence_id}
```

### Artifacts (genérico)

```bash
# Listar todos los artifacts de un run
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts

# Obtener artifact (latest)
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/{type}

# Metadata, revisiones, revision específica
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/{type}/meta
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/{type}/revisions
curl http://localhost:8090/api/v1/runs/{run_id}/artifacts/{type}/v/{revision}
```

### Tipos de artifact válidos

- `service_model` — modelo canónico del servicio (endpoints + refs)
- `model_report` — reporte de confianza y gaps del análisis
- `scenario_plan` — plan de test cases (manual o auto-generado)
- `evidence_pack` — evidencia capturada (req/resp/timing/masking)
- `audit_report` — findings deterministas anclados a evidencia y modelo
- `llm_interpretation` — interpretación LLM bounded con facts, inferences y open_questions anclados

## Ciclo completo de laboratorio

```
1. Crear target (con source.value = path al repo, runtime_hint.base_url)
2. Crear run
3. POST discover (analiza código → service_model + model_report + scenario_plan)
4. (Opcional) Editar scenario_plan manualmente
5. POST execute (runner ejecuta HTTP contra base_url)
6. GET evidence (EvidencePack con items maskeados)
7. POST audit (auditoría determinista → audit_report con findings anclados)
8. POST interpret (LLM bounded → facts, inferences, open_questions, guided_tour)
9. Repetir: editar scenario → re-ejecutar → re-auditar → re-interpretar
```

## Seguridad

- **Header masking**: Authorization, Cookie, Set-Cookie, X-Api-Key, X-Auth-Token → `***MASKED***`
- **SSRF básico**: solo http/https. Bloqueados: file://, gopher://, ftp://, data://
- **Límite de body**: configurable (default 1MB), trunca respuestas grandes

## Estructura (estilo Nexus: handler/dto + usecases/domain + repository/models)

```
toollab-core/
├── cmd/toollab-dashboard/       # Entry point
├── testdata/sample-chi-app/     # Fixture para tests de discovery
├── internal/
│   ├── shared/                  # IDs, clock, hash, refs, errors, httputil
│   ├── target/                  # Dominio: servicios a auditar
│   │   ├── handler/dto/
│   │   ├── usecases/domain/
│   │   └── repository/models/
│   ├── run/                     # Dominio: ejecuciones + orquestación
│   │   ├── handler/dto/
│   │   ├── usecases/domain/
│   │   └── repository/models/
│   ├── artifact/                # Dominio: index + filesystem storage
│   │   ├── handler/dto/
│   │   ├── usecases/domain/
│   │   └── repository/models/
│   ├── discovery/               # Dominio: AST analyzer + scenario generator
│   │   └── usecases/domain/
│   ├── scenario/                # Dominio: ScenarioPlan + ScenarioCase schemas
│   │   └── usecases/domain/
│   ├── runner/                  # Dominio: HTTP runner (ejecuta cases)
│   │   └── usecases/domain/
│   ├── evidence/                # Dominio: ingestor + masking + EvidencePack
│   │   └── usecases/domain/
│   ├── audit/                   # Motor de reglas determinista
│   │   └── usecases/domain/
│   ├── interpretation/          # LLM bounded interpretation
│   │   └── usecases/domain/
│   └── compare/                 # Diff/Trends (Etapa 7)
│       ├── handler/
│       └── usecases/domain/
├── migrations/
└── data/                        # SQLite + artifacts (gitignored)
```

### Compare & Trends (Etapa 7)

```bash
# Métricas de un run (derivadas de evidence_pack)
curl http://localhost:8090/api/v1/runs/{run_id}/metrics

# Comparar dos runs
curl "http://localhost:8090/api/v1/compare?runa={run_a_id}&runb={run_b_id}"

# Trends de un target (series temporales)
curl "http://localhost:8090/api/v1/targets/{target_id}/trends?top_endpoints=10"
```

#### Flujo de comparación

1. Generar dos runs con discover + execute + audit
2. `GET /compare?runa=...&runb=...` devuelve:
   - `service_model_diff`: endpoints agregados/removidos/modificados
   - `audit_diff`: findings nuevos/resueltos/cambiados
   - `evidence_metrics_diff`: métricas overall + regressions por endpoint
3. `GET /targets/{id}/trends` devuelve series con p50/p95, error rate y findings por severity
