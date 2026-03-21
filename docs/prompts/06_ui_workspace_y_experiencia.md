# 06. UI Workspace y Experiencia

## Prerequisito

Leer `01_producto_y_arquitectura.md` y `03_artifacts_dossier_exports.md`.

## Objetivo

Documentar la UI como workspace operativo del laboratorio.

## Alcance obligatorio

- describir vistas reales del frontend
- explicar cómo la UI navega artifacts y progreso
- evitar prometer pantallas o flows inexistentes

## Superficies actuales

- `/targets`: listado y creación de targets
- `/targets/:targetId`: vista principal del run más reciente o nueva ejecución
- tabs: `dashboard`, `endpoints`, `raw`, `documentation`, `audit`
- toggle `EN/ES` para outputs narrativos del runtime LLM

## Responsabilidades

- iniciar `analyze` por SSE y mostrar progreso por step
- presentar `run_mode`, scores y findings de forma accionable
- navegar `endpoint_intelligence` y `endpoint_queries`
- exponer raw QA y documentación derivada del run

## Reglas obligatorias

- la UI debe reflejar si un artifact aún no existe o está deshabilitado
- un tab no debe insinuar evidencia inexistente
- el lenguaje visual puede ser expresivo, pero el significado debe seguir siendo operativo

## Criterios de éxito

- un usuario puede operar el loop completo sin usar cURL manualmente
- la UI muestra claramente el estado del run y la calidad de evidencia

## Orden de ejecución recomendado

1. documentar rutas y tabs reales
2. ligar cada vista con artifacts concretos
3. explicitar límites de consistencia
