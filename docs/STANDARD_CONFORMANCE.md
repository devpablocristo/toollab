# Toolab Standard Conformance (Reusable for Any API)

Este checklist define como adoptar `Toolab Standard v1.1` en cualquier API sin acoplarlo al dominio.

## 1. Scope

- Este documento aplica a cualquier API HTTP.
- El dominio de negocio no cambia el contrato `/_toolab/*`.
- Los nombres/paths de negocio se mantienen fuera del standard.

## 2. Required baseline

Para declararte "Toolab Standard v1.1 conformant", cumplir:

- `GET /_toolab/manifest` implementado y válido.
- `standard_version=1.1`.
- Error envelope estándar en todos los errores.
- Solo anunciar capabilities realmente implementadas.
- No exponer secretos en respuestas `/_toolab/*`.

## 3. Recommended repeatable rollout

1. Implementar `manifest`.
2. Implementar `seed` y `state.fingerprint`.
3. Implementar `profile` como endpoint agregador.
4. Completar `profile` con `suggested_flows`, `invariants`, `limits`, `environment`.
5. Agregar `schema` para predicción de impacto.
6. Mantener `openapi` solo como fallback opcional.

## 4. Reusable acceptance checks

Todos estos checks son agnósticos al dominio:

- `manifest` valida contra `schemas/toolab.standard.manifest.v1.1.schema.json`.
- `profile` valida contra `schemas/toolab.standard.profile.v1.1.schema.json` (si capability anunciada).
- `schema` valida contra `schemas/toolab.standard.schema.v1.1.schema.json` (si capability anunciada).
- `suggested_flows` valida contra `schemas/toolab.standard.suggested_flows.v1.1.schema.json` (si capability anunciada).
- `invariants` valida contra `schemas/toolab.standard.invariants.v1.1.schema.json` (si capability anunciada).
- `limits` valida contra `schemas/toolab.standard.limits.v1.1.schema.json` (si capability anunciada).
- `environment` valida contra `schemas/toolab.standard.environment.v1.1.schema.json` (si capability anunciada).
- Errores validan contra `schemas/toolab.standard.error.v1.1.schema.json`.

## 5. Conformance report

La salida de validación de una API `SHOULD` persistirse como JSON y validar contra:

- `schemas/toolab.standard.conformance_report.v1.1.schema.json`

Este reporte permite repetir auditoría y comparar APIs distintas con la misma métrica de cumplimiento.

## 6. Minimal cURL smoke script

```bash
set -euo pipefail
BASE_URL="${BASE_URL:-http://localhost:8080}"

curl -fsS "${BASE_URL}/_toolab/manifest" > /tmp/manifest.json
if jq -e '.capabilities | index("profile")' /tmp/manifest.json >/dev/null; then
  curl -fsS "${BASE_URL}/_toolab/profile" > /tmp/profile.json
fi
```

## 7. Pass criteria

- La API declara capacidades coherentes con endpoints reales.
- Toolab puede construir escenario con `profile` sin conocimiento del dominio.
- Toolab puede ejecutar, recolectar evidencia y producir fingerprint determinista.
- OpenAPI solo se usa si `profile` no alcanza para cubrir endpoints/flows.
