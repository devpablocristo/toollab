# 07. Testing, Validación y Release

## Prerequisito

Leer `00_base_transversal.md` a `06_ui_workspace_y_experiencia.md`.

## Objetivo

Definir cómo validar cambios en ToolLab antes de considerarlos consistentes.

## Alcance obligatorio

- cubrir backend Go, frontend TypeScript y consistencia documental
- exigir evidencia de build/test para cambios sustantivos
- verificar alineación entre código, prompts y documentación

## Gates mínimos

### Backend

- `go test ./...`
- `go build ./cmd/toollab-dashboard`

### Frontend

- `npm run build`

### Documentación y prompts

- revisar que nombres, pipeline, artifacts, rutas y estados runtime coincidan con el código
- si se externalizan prompts, validar que el backend siga compilando y cargándolos

## Reglas obligatorias

- no afirmar que una capability está activa si el código no la genera
- no cerrar trabajo de docs/prompts con drift factual
- todo cambio que toque runtime LLM debe validar build/test del backend

## Criterios de éxito

- el repo compila
- la suite de prompts describe el producto real
- la documentación raíz y los READMEs quedaron alineados entre sí

## Orden de ejecución recomendado

1. validar backend
2. validar frontend
3. hacer revisión final de consistencia factual
