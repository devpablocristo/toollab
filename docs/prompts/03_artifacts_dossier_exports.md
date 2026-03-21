# 03. Artifacts, Dossier y Exports

## Prerequisito

Leer `00_base_transversal.md` y `02_pipeline_de_analisis.md`.

## Objetivo

Definir los artifacts como contrato operativo del producto.

## Alcance obligatorio

- listar artifacts principales y su propósito
- explicar diferencia entre artifacts crudos, compactados y derivados
- documentar exports listos para uso humano

## Tipos de outputs

### Artifacts estructurales

- `target_profile`
- `endpoint_catalog`
- `router_graph`
- `ast_entities`
- `ast_code_patterns`

### Artifacts de evidencia y análisis

- `schema_registry`
- `inferred_contracts`
- `semantic_annotations`
- `smoke_results`
- `auth_matrix`
- `fuzz_results`
- `logic_results`
- `abuse_results`
- `confirmations`
- `findings_raw`
- `error_signatures`
- `raw_evidence`

### Artifacts de síntesis

- `run_summary`
- `scoring`
- `dossier_full`
- `dossier_docs_mini`
- `dossier_llm`

### Artifacts de consumo operativo

- `endpoint_intelligence`
- `endpoint_intelligence_index`
- `endpoint_queries`
- `postman_collection`
- `curl_book`
- `env_example`
- `run_summary_export`

### Artifacts LLM

- `llm_docs`
- `llm_audit`

## Reglas obligatorias

- `dossier_full` es la síntesis rica del run
- `dossier_docs_mini` y `dossier_llm` existen para bounded prompting, no para reemplazar el dossier
- exports como `curl_book` o `postman_collection` deben derivarse de intelligence/evidence, no inventarse aparte

## Criterios de éxito

- queda claro qué artifact debe leer cada consumidor
- se evita usar `raw_evidence` como única interfaz humana cuando ya existe una síntesis mejor

## Orden de ejecución recomendado

1. separar artifacts por capa
2. explicar dossiers compactados
3. cerrar con exports y consumidores
