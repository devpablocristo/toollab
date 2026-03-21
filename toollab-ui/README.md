# ToolLab UI

Frontend React/TypeScript del laboratorio ToolLab. Consume `toollab-core` y ofrece un workspace para lanzar análisis, revisar evidencia, navegar endpoints y leer documentación derivada del run.

## Quickstart

```bash
# 1. Levantar backend
cd ../toollab-core
CGO_ENABLED=1 go run ./cmd/toollab-dashboard

# 2. Instalar dependencias del frontend
cd ../toollab-ui
npm install

# 3. Levantar Vite
npm run dev
```

Abrir [http://localhost:5173](http://localhost:5173)

## Configuración

| Variable | Default | Descripción |
|---|---|---|
| `VITE_API_BASE_URL` | vacío | Base URL del backend. En dev, Vite proxea `/api` a `localhost:8090`. |

## Superficie actual

- `/targets`: listado y creación de targets
- `/targets/:targetId`: vista principal del target con último run y re-analyze
- `Dashboard`: scores, `run_mode`, findings y métricas agregadas
- `Endpoints`: `endpoint_intelligence` e instrucciones por endpoint
- `Raw QA`: artifacts crudos y exploración operativa
- `Documentation`: render de `llm_docs`
- `Audit`: render de `llm_audit` cuando exista

## Comportamiento relevante

- `Analyze` usa SSE contra `POST /api/v1/targets/{target_id}/analyze`
- el idioma `EN/ES` afecta el output narrativo del runtime LLM
- documentación y auditoría se consultan por polling hasta que el artifact exista o falle
- la UI consume `run_summary`, `endpoint_intelligence_index`, `endpoint_queries` y artifacts tipados

## Stack

React 18 + TypeScript + Vite 5 + TailwindCSS 3 + React Router 6 + TanStack Query 5 + React Markdown

## Build

```bash
npm run build
```
