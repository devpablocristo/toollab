# 01. Producto y Arquitectura

## Prerequisito

Leer `00_base_transversal.md`.

## Objetivo

Dejar inequívoco qué es `toollab`, qué problema resuelve y cómo se divide entre backend y frontend.

## Alcance obligatorio

- definir a ToolLab como laboratorio de análisis para APIs/servicios/repos
- explicar la relación entre `target`, `run`, `artifact` y `workspace`
- aclarar ownership de `toollab-core` y `toollab-ui`
- fijar la taxonomía canónica: `IntelligenceService` + `SynthesisService`
- dejar explícito que ToolLab no es un agente conversacional del producto

## Modelo del producto

- `target`: servicio o API a analizar; combina repo local y `runtime_hint`
- `run`: ejecución concreta del pipeline con seed, budget y evidencia acumulada
- `artifact`: output persistido y reutilizable de una etapa o postproceso
- `workspace`: superficie UI para navegar un run y profundizar manualmente

## Arquitectura

### Backend

- `toollab-core` expone HTTP API en `:8090`
- persiste metadatos en SQLite
- guarda artifacts en filesystem
- orquesta pipeline determinista y generación de exports (`IntelligenceService`)
- ejecuta runtime LLM bounded sobre dossiers compactados (`SynthesisService`)

### Frontend

- `toollab-ui` consume la API y opera como workspace de análisis
- lanza `analyze` por SSE y muestra progreso por step
- navega `dashboard`, `endpoints`, `raw`, `documentation` y `audit`

## Ownership

- `toollab-core` es dueño de los artifacts y del significado del pipeline
- `toollab-ui` es dueño de la experiencia de exploración y operación del laboratorio
- la documentación raíz describe el producto completo, no solo una app individual

## Taxonomía del ecosistema

- `IntelligenceService`: derivaciones determinísticas post-run como `endpoint_intelligence`, índices y query scripts
- `SynthesisService`: artefactos LLM batch como `llm_docs` y `llm_audit`
- ToolLab no se modela como `ProductAgent`, `DomainAgent`, `CopilotAgent` ni assistant multi-turn

## Drift a evitar

- presentar ToolLab como simple scanner de seguridad
- presentarlo como simple generador de docs
- omitir el rol de `AST + runtime + artifacts`
- documentar auditoría LLM como activa por default cuando hoy está reservada pero deshabilitada en runtime

## Criterios de éxito

- la narrativa deja claro por qué ToolLab combina análisis estático, ejecución y LLM bounded
- queda claro que la fuente de verdad no es el prompt sino la evidencia

## Orden de ejecución recomendado

1. definir entidades del producto
2. definir arquitectura y ownership
3. explicitar límites y evitar claims ambiguos
