# ToolLab v2 (Backend)

Backend nuevo, AST-first y determinista, separado del stack anterior.

## Ejecutar

```bash
cd /home/pablo/Projects/Pablo/toollab/toollab-v2
go run ./cmd/toollab-api
```

API por defecto en `http://localhost:8090`.

## Endpoints base

- `POST /v1/runs`
- `GET /v1/runs`
- `GET /v1/runs/{id}`
- `GET /v1/runs/{id}/model`
- `GET /v1/runs/{id}/summary`
- `GET /v1/runs/{id}/audit`
- `GET /v1/runs/{id}/scenarios`
- `GET /v1/runs/{id}/llm`
- `GET /v1/runs/{id}/artifacts`
- `GET /v1/runs/{id}/logs`

## Crear run (ejemplo)

```bash
curl -sX POST http://localhost:8090/v1/runs \
  -H 'content-type: application/json' \
  -d '{
    "source_type":"local_path",
    "local_path":"/home/pablo/Projects/Pablo/nexus",
    "llm_enabled": false
  }' | jq
```
