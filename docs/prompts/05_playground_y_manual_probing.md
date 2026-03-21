# 05. Playground y Manual Probing

## Prerequisito

Leer `00_base_transversal.md` y `02_pipeline_de_analisis.md`.

## Objetivo

Definir el rol del playground como superficie manual de exploración controlada.

## Alcance obligatorio

- explicar `send`, `replay` y `auth-profiles`
- documentar SSRF control, timeout y captura de evidencia
- dejar claro que el playground complementa, no reemplaza, el pipeline

## Reglas obligatorias

- toda interacción manual debe quedar trazada como evidencia reutilizable
- `auth profiles` se usan para validar hipótesis de auth sin exponer secretos en claro
- `replay` sirve para reproducibilidad sobre evidencia previa
- el allowed host del run restringe el alcance del probing

## Riesgos a evitar

- convertir el playground en cliente HTTP genérico sin límites
- permitir hosts arbitrarios o protocolos inseguros
- usar resultados manuales fuera del contexto del run que los originó

## Criterios de éxito

- el usuario entiende cuándo usar pipeline automático y cuándo usar playground
- queda claro cómo manual probing mejora cobertura y confirmación

## Orden de ejecución recomendado

1. describir capabilities
2. documentar límites de seguridad
3. conectar playground con evidencia y confirmación
