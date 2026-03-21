# 04. LLM Runtime y Prompting

## Prerequisito

Leer `00_base_transversal.md`, `02_pipeline_de_analisis.md` y `03_artifacts_dossier_exports.md`.

## Objetivo

Formalizar cómo ToolLab usa LLM sin romper el principio `evidence-first`.

## Alcance obligatorio

- documentar prompts de documentación y auditoría
- documentar `offline` y `online_partial` prefixes
- documentar externalización/versionado de prompts
- aclarar el estado actual del runtime

## Reglas de prompting

- el LLM consume artifacts compactados, no el repo completo ni evidencia sin curar
- el prompt debe degradar la narrativa según `run_mode`
- toda afirmación sin evidencia debe transformarse en `gap`, `open_question` o `inconclusive`

## Prompts obligatorios

- `docs_prompt`: guía de integración para desarrolladores basada en evidencia
- `audit_prompt`: diagnóstico AppSec/API quality con JSON schema fijo
- `offline_docs_prefix` y `offline_audit_prefix`
- `partial_docs_prefix` y `partial_audit_prefix`
- sufijos de idioma para español

## Externalización

- los textos base deben vivir en archivos del repo, no hardcodeados en constantes gigantes
- el runtime puede componer `prefix + prompt + suffix + dossier`
- cualquier cambio de prompt debe ser reviewable como diff normal

## Estado actual

- la documentación LLM está activa
- el prompt de auditoría existe y debe mantenerse actualizado
- la generación de `llm_audit` sigue deshabilitada por default en el runtime actual para controlar costo/token usage

## Criterios de éxito

- los prompts quedan editables sin tocar lógica principal
- el comportamiento del LLM queda alineado con `run_mode`
- documentación y código describen el mismo estado real

## Orden de ejecución recomendado

1. fijar contrato del runtime LLM
2. definir prompts y prefijos
3. externalizar textos
4. documentar límites y estado actual
