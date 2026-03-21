# 10. Emergent Behavior Audit and Scoring

## Prerequisito

Leer `08_behavioral_simulation_foundations.md` y `09_multiagent_scenarios_and_policies.md`.

## Objetivo

Definir cómo ToolLab convierte una simulación multiagente en evidencia, findings, scoring y recomendaciones accionables.

## Alcance obligatorio

- definir métricas y artifacts
- formalizar findings de comportamiento emergente
- definir scoring y comparativas entre runs, versiones o policies

## Qué debe medir ToolLab

- convergencia o divergencia
- loops
- deadlocks
- starvation
- abuso de recursos
- escalada de permisos
- colisión de objetivos
- degradación por cooperación oportunista
- sensibilidad a latencia, falla parcial o escasez
- blast radius de una decisión o actor

## Artifacts sugeridos

- `behavior_trace`
- `actor_timeline`
- `resource_usage_matrix`
- `policy_violations`
- `emergent_findings`
- `simulation_summary`
- `simulation_replay_bundle`

Si se crean, deben seguir la misma disciplina que el resto de los artifacts de ToolLab: ser tipados, versionables y reutilizables por UI, exports y reporting.

## Tipos de findings

### Riesgo sistémico

- un patrón emergente afecta estabilidad, seguridad o gobernanza del sistema

### Riesgo por incentivo

- la política o diseño induce conducta oportunista, no deseada o costosa

### Riesgo por acoplamiento

- interacciones entre actores o servicios producen fallos en cascada

### Riesgo por contención insuficiente

- el sistema no aísla adecuadamente daño, retries, permisos o consumo

## Reglas obligatorias

### E1. Findings con evidencia

- un finding debe apuntar a eventos, trazas, timelines o métricas concretas
- no alcanza con “parece inestable”; se necesitan señales observables

### E2. Comparabilidad

- el scoring debe poder compararse entre:
  - dos versiones del sistema
  - dos policies
  - dos configuraciones de recursos
  - dos perfiles de actores

### E3. Replay

- una simulación relevante debe poder re-ejecutarse o al menos inspeccionarse mediante replay

### E4. Remediación accionable

- cada finding importante debe terminar en recomendaciones de diseño, policy, aislamiento, quotas o workflow

## Scoring sugerido

- `stability`
- `governance`
- `resource_control`
- `policy_effectiveness`
- `resilience_under_conflict`
- `abuse_resistance`

Cada score debe incluir:

- valor
- rationale
- evidence refs
- nivel de confianza

## Criterios de éxito

- la simulación produce más que visualizaciones: produce evidencia fuerte
- los findings permiten decidir cambios concretos en producto o arquitectura
- ToolLab gana una capacidad diferencial de auditoría conductual

## Orden de ejecución recomendado

1. definir métricas base
2. definir artifacts y replay
3. definir findings y scoring
4. definir comparativas y remediación
