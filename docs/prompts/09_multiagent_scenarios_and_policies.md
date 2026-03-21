# 09. Multi-Agent Scenarios and Policies

## Prerequisito

Leer `08_behavioral_simulation_foundations.md`.

## Objetivo

Definir cómo ToolLab modela escenarios multiagente, recursos escasos, restricciones y policies para provocar y estudiar interacciones complejas.

## Alcance obligatorio

- describir tipos de actores y roles
- definir escenarios reproducibles
- formalizar policies y restricciones de entorno
- cubrir cooperación, conflicto y abuso

## Tipos de actores

### `agents`

- persiguen objetivos
- observan estado parcial
- eligen acciones
- pueden aprender, ajustar o seguir estrategias

### `services`

- exponen interfaces y cambian estado
- consumen recursos
- pueden degradarse, rechazar, rate-limit o fallar parcialmente

### `operators`

- aplican acciones administrativas o de control
- alteran políticas, cuotas o permisos

### `attackers` y `faulty actors`

- intentan explotar huecos
- fuerzan retries
- generan inputs inválidos
- presionan recursos o policies

## Escenarios mínimos

- cooperación sana
- competencia por recursos
- actor oportunista
- actor malicioso
- dependencia lenta o caída parcial
- cascada de reintentos
- conflicto de objetivos entre actores
- policy inconsistente
- quotas insuficientes o mal calibradas

## Policies y restricciones

Las simulaciones deben permitir definir:

- permisos por actor
- quotas por recurso
- budget por tiempo o costo
- límites de llamadas o tool use
- aislamiento entre actores
- reglas de retry, backoff y circuit breaking
- reglas de compensación o bloqueo

## Reglas obligatorias

### E1. Escenario explícito

- ninguna simulación corre “libre”; siempre debe existir un escenario definido
- el escenario describe actores, recursos, objetivos, restricciones y condiciones de éxito/fallo

### E2. Policies auditables

- toda policy debe ser serializable y comparable entre runs
- el cambio de una policy debe poder medirse contra baseline

### E3. Entorno hostil realista

- no asumir condiciones ideales
- incluir degradación, escasez, ruido, conflicto y fallas parciales

## Outputs esperados

- timeline de decisiones
- trazas por actor
- consumo de recursos
- policy violations
- loops o divergence signals
- eventos de cooperación, conflicto o starvation

## Criterios de éxito

- los escenarios sirven para descubrir riesgo, no solo para visualizar actividad
- policies y restricciones quedan suficientemente formales para compararse entre versiones

## Orden de ejecución recomendado

1. definir actor taxonomy
2. definir escenarios
3. definir policies, budgets y failure modes
4. definir outputs observables
