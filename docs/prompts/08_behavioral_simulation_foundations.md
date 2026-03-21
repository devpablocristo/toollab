# 08. Behavioral Simulation Foundations

## Prerequisito

Leer `00_base_transversal.md`, `02_pipeline_de_analisis.md` y `03_artifacts_dossier_exports.md`.

## Objetivo

Definir una línea avanzada de ToolLab para simular actores autónomos, servicios y restricciones operativas con el fin de observar, medir y auditar comportamiento emergente.

## Alcance obligatorio

- describir esta capability como `behavioral simulation`, no como promesa filosófica de “vida digital”
- mantener el principio `evidence-first`
- integrarla al modelo existente de `run`, `artifact`, `scoring` y `report`

## Definición operativa

En ToolLab, esta capability se define así:

un sistema de simulación de entidades computacionales y servicios dentro de un entorno con reglas, recursos, objetivos, tiempo y restricciones, para descubrir conductas emergentes, riesgos sistémicos y efectos no triviales de interacción.

## Qué se modela

### Entidades

- `human-like actors` simulados
- `agents`
- `services`
- `operators`
- `batch jobs`
- `attackers`
- `faulty consumers`

### Entorno

- tiempo discreto o continuo
- recursos limitados
- cuotas
- colas
- latencias
- fallas parciales
- políticas y permisos

### Interacciones

- llamadas HTTP
- tool calling
- consumo de recursos
- cambios de estado
- coordinación o conflicto
- reintentos
- escaladas

## Principios obligatorios

### E1. No humo

- no hablar de conciencia, intención fuerte ni equivalencia biológica
- el valor del módulo es descubrir comportamiento y riesgo, no hacer una demo conceptual

### E2. Reproducibilidad

- toda simulación debe aceptar seed, configuración y escenario explícitos
- el mismo escenario debe poder re-ejecutarse para comparar resultados

### E3. Integración con ToolLab

- una simulación debe ser un tipo de `run` o sub-run compatible con artifacts existentes o nuevos artifacts tipados
- los hallazgos deben terminar en evidencia utilizable por scoring y reporting

### E4. Observabilidad nativa

- cada actor y cada interacción relevante debe quedar trazada
- loops, deadlocks, starvation, escalada de permisos y abuso de recursos deben ser detectables

## Resultado esperado

Esta capability debe permitir que ToolLab deje de auditar solo artefactos estáticos y pase a auditar también dinámicas de interacción.

## Criterios de éxito

- el módulo se entiende como laboratorio de comportamiento, no como gimmick
- el resultado es medible, reproducible y accionable
- queda claro cómo se conecta con el resto del producto

## Orden de ejecución recomendado

1. definir actores, entorno y recursos
2. definir ejecución reproducible
3. definir artifacts y scoring derivados
