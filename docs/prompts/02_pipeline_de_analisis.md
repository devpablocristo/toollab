# 02. Pipeline de Análisis

## Prerequisito

Leer `00_base_transversal.md` y `01_producto_y_arquitectura.md`.

## Objetivo

Documentar el pipeline real de ToolLab y las responsabilidades de cada step.

## Alcance obligatorio

- usar el orden real del pipeline
- explicar inputs, outputs y límites de cada etapa
- conectar steps con `budget`, `run_mode` y findings

## Pipeline real

`preflight -> astdiscovery -> schema -> smoke -> authmatrix -> fuzz -> logic -> abuse -> confirm -> report`

## Responsabilidad por etapa

### `preflight`

- valida que exista `base_url`
- normaliza conectividad inicial
- detecta `health endpoints`, `content types`, redirects y hints de auth

### `astdiscovery`

- descubre endpoints desde el repo
- construye refs a handlers, rutas y entidades AST

### `schema`

- infiere contratos y registros de schema aprovechables por etapas posteriores

### `smoke`

- ejecuta happy-path probes básicos
- determina si el servicio realmente responde

### `authmatrix`

- compara comportamiento con y sin auth
- alimenta readiness y exposiciones

### `fuzz`

- ejecuta probing guiado y acotado por budget

### `logic`

- busca inconsistencias de negocio y señales semánticas

### `abuse`

- busca abuso funcional, resiliencia y comportamientos problemáticos

### `confirm`

- consolida señales y aumenta o degrada confianza

### `report`

- genera findings derivados, scoring, dossier y exports

## Reglas obligatorias

- una etapa nueva no puede entrar sin budget y criterio de corte
- fallas tempranas en `preflight` o `astdiscovery` pueden invalidar el run
- `report` no inventa: agrega, resume y exporta lo que el run ya observó

## Criterios de éxito

- cualquier lector entiende qué produce cada etapa y por qué existe
- queda claro cómo el pipeline termina en artifacts navegables y no solo en texto final

## Orden de ejecución recomendado

1. fijar el orden real del pipeline
2. describir inputs/outputs por step
3. ligar cada step a artifacts y run modes
