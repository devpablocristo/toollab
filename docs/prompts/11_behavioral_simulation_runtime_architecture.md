# 11. Behavioral Simulation Runtime Architecture

## Prerequisito

Leer `08_behavioral_simulation_foundations.md`, `09_multiagent_scenarios_and_policies.md` y `10_emergent_behavior_audit_and_scoring.md`.

## Objetivo

Definir una arquitectura técnica concreta para implementar la capacidad de `behavioral simulation` dentro de ToolLab sin romper el modelo actual de `targets`, `runs`, `artifacts` y `reports`.

## Alcance obligatorio

- definir cómo entra esta capability en `toollab-core`
- proponer módulos, contracts, artifacts y endpoints
- dejar explícito qué parte es futura y qué parte ya existe hoy

## Estado actual vs estado objetivo

### Estado actual

- ToolLab ya tiene `target`, `run`, `artifact`, pipeline determinista y reporting basado en evidencia
- hoy audita principalmente código, endpoints, runtime HTTP y derivados LLM
- no existe todavía un runtime dedicado de simulación multiagente

### Estado objetivo

- agregar un runtime de simulación como capability avanzada
- reutilizar storage, scoring, reporting y UI donde sea posible
- mantener separación clara entre análisis HTTP clásico y simulación conductual

## Diseño recomendado

### Opción preferida: `run kind`

Mantener un único modelo de `run`, pero agregar un `run_kind`:

- `analysis`
- `behavioral_simulation`

Ventajas:

- reutiliza `artifacts`, `run_summary`, baseline y compare
- evita crear un segundo sistema paralelo
- permite UI y reporting unificados

### Opción alternativa: `sub-run`

Modelar simulación como un `sub-run` asociado a un run de análisis.

Ventajas:

- separa mejor la ejecución

Desventajas:

- complica navegación, baseline y ownership

Regla:

- arrancar con `run kind`; solo migrar a `sub-run` si el volumen o semántica crecen demasiado

## Módulos sugeridos en `toollab-core`

```text
internal/
├── simulation/
│   ├── handler.go
│   ├── usecases.go
│   ├── repository.go
│   ├── runtime.go
│   ├── actor_registry.go
│   ├── scenario_registry.go
│   ├── policy_engine.go
│   ├── replay.go
│   └── simulation/
│       └── dto/
```

### Responsabilidades

- `handler.go`: API HTTP para crear, listar, ejecutar y consultar simulaciones
- `usecases.go`: orquestación de simulation run, validación, budgets y persistencia
- `runtime.go`: motor de ticks/eventos/interacciones
- `actor_registry.go`: catálogo de tipos de actor soportados
- `scenario_registry.go`: escenarios predefinidos y plantillas
- `policy_engine.go`: quotas, permisos, aislamiento, retries, bloqueos
- `replay.go`: reconstrucción de timeline y re-ejecución controlada

## Entidades técnicas sugeridas

### `SimulationSpec`

- `schema_version`
- `scenario_id`
- `seed`
- `duration_budget`
- `tick_limit`
- `actors[]`
- `resources`
- `policies`
- `failure_injection`
- `success_criteria`

### `SimulationActor`

- `actor_id`
- `actor_type`
- `role`
- `goal`
- `strategy`
- `permissions`
- `resource_budget`
- `initial_state`

### `SimulationEvent`

- `event_id`
- `tick`
- `timestamp`
- `actor_id`
- `target_ref`
- `action_type`
- `request_or_command`
- `result`
- `resource_delta`
- `policy_decision`

### `SimulationOutcome`

- `status`
- `termination_reason`
- `stability_signal`
- `risk_flags`
- `summary_metrics`

## Artifacts recomendados

- `simulation_spec`
- `behavior_trace`
- `actor_timeline`
- `resource_usage_matrix`
- `policy_violations`
- `emergent_findings`
- `simulation_summary`
- `simulation_replay_bundle`

Reglas:

- deben ser `ArtifactType` tipados
- deben seguir versionado y storage igual que los artifacts actuales
- deben ser consumibles por UI, compare y exportes

## Endpoints sugeridos

### Arranque mínimo

- `POST /api/v1/targets/{target_id}/simulate`
- `GET /api/v1/runs/{run_id}/artifact/simulation_summary`
- `GET /api/v1/runs/{run_id}/artifact/behavior_trace`
- `GET /api/v1/runs/{run_id}/artifact/policy_violations`
- `GET /api/v1/runs/{run_id}/artifact/emergent_findings`

### Expansión

- `GET /api/v1/runs/{run_id}/simulation/replay`
- `GET /api/v1/runs/{run_id}/simulation/actors`
- `GET /api/v1/runs/{run_id}/simulation/timeline/{actor_id}`

## Integración con scoring y compare

- un simulation run debe producir `scores_available`
- los scores de simulación deben coexistir con los scores de análisis clásico
- compare debe soportar:
  - run vs run del mismo tipo
  - policy A vs policy B
  - scenario A vs scenario B

## Integración con UI

La UI no debe mezclar tabs viejos con significado nuevo sin aclararlo.

Agregar, cuando se implemente:

- tab `Simulation`
- timeline de actores
- mapa de consumo de recursos
- panel de policy violations
- summary de findings emergentes

## Reglas obligatorias

### E1. Capability futura explícita

- esta arquitectura describe una extensión futura de ToolLab
- no debe documentarse como implementada hasta que exista código y evidencia

### E2. Reutilizar antes que bifurcar

- preferir extender `run`, `artifact`, `compare`, `report`
- evitar un subsistema aislado que duplique persistencia y reporting

### E3. Runtime controlado

- simulaciones con budgets, tick limit, timeout y seed
- no permitir ejecución abierta ni no determinista por default

### E4. Replay primero

- si no hay replay o trazabilidad suficiente, la simulación pierde valor de auditoría

## Criterios de éxito

- la capacidad se puede implementar por etapas
- el diseño preserva cohesión con el resto de ToolLab
- queda claro cómo pasar de idea a backend real sin rehacer el producto

## Orden de ejecución recomendado

1. agregar `run kind`
2. agregar nuevos `ArtifactType`
3. crear `simulation` module con spec + runtime mínimo
4. exponer artifacts por API
5. sumar UI específica
6. recién después agregar actores adaptativos más sofisticados
