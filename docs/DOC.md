# ToolLab Docs

## Qué es

ToolLab es una herramienta para analizar, auditar, documentar y entender APIs a partir de evidencia real. Combina inspección estática del repo, ejecución HTTP controlada y generación de artifacts versionables que luego alimentan documentación, QA y flujos de investigación.

Dentro de la taxonomía del ecosistema AI, ToolLab se expresa como:

- `IntelligenceService` para derivaciones determinísticas sobre dossier, endpoints y evidencia
- `SynthesisService` para artefactos LLM batch acotados por evidencia

## Qué no es

- no depende de un único prompt gigante
- no reemplaza un pentest manual profundo
- no debe inventar contratos ni comportamiento cuando el servicio no respondió
- no usa LLM como fuente primaria de verdad: el ancla son AST, evidence y artifacts

## Arquitectura del producto

- `toollab-core`: backend Go con persistencia SQLite, artifacts en filesystem, pipeline determinista, `IntelligenceService` y runtime LLM bounded (`SynthesisService`)
- `toollab-ui`: workspace React para targets, runs, dashboard, endpoint intelligence, documentación y raw QA
- `docker-compose.yml`: forma recomendada de levantar ambos componentes

## Flujo operativo

1. Crear `target` con path al repo y `runtime_hint.base_url`
2. Lanzar `analyze`
3. Ejecutar pipeline completo
4. Guardar artifacts estructurados
5. Navegar el run desde UI o API
6. Repetir con nueva evidencia para mejorar cobertura y confianza

## Pipeline real

El pipeline actual es:

`preflight -> astdiscovery -> schema -> smoke -> authmatrix -> fuzz -> logic -> abuse -> confirm -> report`

Notas clave:

- `preflight` normaliza `base_url`, hace probes básicos y detecta hints operativos
- `astdiscovery` construye catálogo de endpoints y refs de código
- `schema`, `smoke`, `authmatrix`, `fuzz`, `logic` y `abuse` generan evidencia y findings candidatos
- `confirm` consolida señales y `report` genera scoring, dossier y exports
- la documentación LLM corre en background sobre artifacts compactados como `SynthesisService`
- el prompt de auditoría LLM está definido, pero su generación sigue deshabilitada por default en el runtime actual

## Outputs principales

- `target_profile`
- `endpoint_catalog`, `router_graph`, `ast_entities`, `ast_code_patterns`
- `schema_registry`, `inferred_contracts`, `semantic_annotations`
- `smoke_results`, `auth_matrix`, `fuzz_results`, `logic_results`, `abuse_results`
- `confirmations`, `findings_raw`, `error_signatures`, `raw_evidence`
- `run_summary`, `scoring`, `dossier_full`, `dossier_docs_mini`, `dossier_llm`
- `endpoint_intelligence`, `endpoint_queries`, `postman_collection`, `curl_book`
- `llm_docs` y, cuando se habilite, `llm_audit`

## Principio rector

Todo output narrativo debe respetar el modo de evidencia del run:

- `offline`: explicar límites, no afirmar comportamiento runtime
- `online_partial`: marcar incertidumbre y gaps
- `online_good` / `online_strong`: usar evidencia suficiente, igual sin inventar

## Suite oficial de prompts

La documentación ejecutiva de ToolLab vive en `docs/prompts/`. Esa suite define:

- invariantes del producto
- arquitectura y ownership
- pipeline y artifacts
- runtime LLM y reglas de prompting
- playground/manual probing
- UI/workspace
- testing, validación y release gates

## Línea avanzada

ToolLab también puede evolucionar hacia una capacidad avanzada de `behavioral simulation`:

- simulación de actores autónomos, servicios y restricciones operativas
- generación de evidencia sobre cooperación, conflicto, loops, abuso y degradación
- auditoría de comportamiento emergente antes de producción

Esto no debe presentarse como “vida digital” en sentido filosófico, sino como un laboratorio reproducible para `multi-agent systems`, `autonomous workflows` y ecologías de servicios bajo reglas, recursos y policies.

Si se implementa, la recomendación es integrarlo como un `run kind` nuevo dentro de ToolLab, reutilizando `artifacts`, `compare`, `scoring` y `reporting`, en vez de crear un subsistema completamente separado.
