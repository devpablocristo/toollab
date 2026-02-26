# Toollab Standard v1.1 (Normative)

Este documento define el contrato extendido `/_toollab/*` para aplicaciones `toollab-ready`.
La meta es interoperabilidad fuerte entre cualquier adapter y cualquier cliente Toollab-compatible.

## 1. Scope

- Este standard extiende [SPEC_ADAPTER.md](/home/pablo/Projects/Pablo/toollab/docs/SPEC_ADAPTER.md) sin romper compatibilidad.
- `SPEC_ADAPTER.md` define el core (manifest, state, seed, metrics, traces, logs).
- Este documento agrega capacidades para discovery avanzado: `profile`, `schema`, `openapi`, `suggested_flows`, `invariants`, `limits`, `environment`.

## 2. Normative language

Las palabras normativas siguen RFC 2119:

- `MUST` / `MUST NOT`: obligatorio.
- `SHOULD` / `SHOULD NOT`: recomendado fuerte.
- `MAY`: opcional.

## 3. Versioning and compatibility

- El prefijo de rutas `/_toollab/` `MUST` mantenerse.
- `adapter_version` en `manifest` `MUST` seguir siendo `"1"` en esta versión.
- El campo nuevo `standard_version` `MUST` existir y `MUST` ser `"1.1"` para implementaciones de este standard.
- Un cliente `MUST` tratar capacidades desconocidas como no soportadas.
- Un servidor `MUST NOT` anunciar una capability si no implementa su endpoint asociado.

## 4. Security and exposure

- Endpoints `/_toollab/*` `MUST` estar explícitamente habilitados (opt-in).
- En producción, la exposición pública `MUST NOT` ser el default.
- El adapter `SHOULD` requerir autenticación/ACL separada del tráfico normal de negocio.
- El adapter `MUST NOT` exponer secretos en respuestas (`tokens`, credentials, DSN, etc.).
- El adapter `MUST` limitar tamaño de payload y costo de cómputo por request.

## 5. Common HTTP requirements

- Todas las respuestas JSON `MUST` usar `Content-Type: application/json`.
- El encoding `MUST` ser UTF-8.
- `GET` `MUST` ser side-effect free.
- Cuando un recurso no está habilitado por capacidad, el servidor `MUST` responder `404` o `501`.
- Cuando hay falla temporal de backend, `SHOULD` responder `503`.
- Todos los errores JSON `MUST` seguir:

```json
{
  "error": "codigo_estandar",
  "message": "descripcion legible"
}
```

- El payload de error `SHOULD` validar contra `schemas/toollab.standard.error.v1.1.schema.json`.

## 6. Manifest (required)

Endpoint:

```http
GET /_toollab/manifest
```

Requisitos:

- Este endpoint `MUST` existir.
- Respuesta `MUST` validar contra `schemas/toollab.standard.manifest.v1.1.schema.json`.
- `capabilities` `MUST` contener valores únicos.
- `links` `MAY` existir para discovery directo.

Ejemplo:

```json
{
  "adapter_version": "1",
  "standard_version": "1.1",
  "app_name": "sample-api",
  "app_version": "2026.02.0",
  "capabilities": [
    "state.fingerprint",
    "state.snapshot",
    "state.restore",
    "state.reset",
    "seed",
    "metrics",
    "traces",
    "logs",
    "profile",
    "schema",
    "openapi",
    "suggested_flows",
    "invariants",
    "limits",
    "environment"
  ],
  "links": {
    "openapi_url": "http://localhost:8080/openapi.yaml",
    "schema_url": "http://localhost:8080/_toollab/schema",
    "profile_url": "http://localhost:8080/_toollab/profile",
    "suggested_flows_url": "http://localhost:8080/_toollab/suggested_flows",
    "invariants_url": "http://localhost:8080/_toollab/invariants",
    "limits_url": "http://localhost:8080/_toollab/limits",
    "environment_url": "http://localhost:8080/_toollab/environment"
  }
}
```

Regla API-agnostic:

- Este contrato `MUST` ser reusable para cualquier dominio (payments, auth, crm, iot, etc.).
- Ningún endpoint del standard `MUST` exigir campos de negocio específicos de una API concreta.
- Cualquier metadata de dominio `MAY` viajar en `description`, `params` o `metadata` opcionales, sin romper schemas base.

## 7. New capabilities

### 7.1 `profile` (recommended primary discovery)

Endpoint:

```http
GET /_toollab/profile
```

Discovery alternativo:

- `links.profile_url` `MAY` apuntar a esa misma vista agregada en URL absoluta.

Contrato:

- Respuesta `MUST` validar contra `schemas/toollab.standard.profile.v1.1.schema.json`.
- `profile` consolida discovery en un solo payload y `SHOULD` incluir, cuando existan:
  - `schema`
  - `suggested_flows`
  - `invariants`
  - `limits`
  - `environment`
  - metadata de `openapi` (url/etag/content_type) si está disponible
- Si una subsección no está disponible, `profile` `SHOULD` omitirla y `MAY` reportarla en `unknowns`.
- Si `profile` no está disponible temporalmente, `SHOULD` devolver `503 profile_not_available`.

Uso esperado:

- Punto único de ingesta para auditoría Toollab.
- Reduce round-trips y deriva en evidencia más consistente.

### 7.2 `schema`

Endpoint:

```http
GET /_toollab/schema
```

Contrato:

- Respuesta `MUST` validar contra `schemas/toollab.standard.schema.v1.1.schema.json`.
- `database.type` `MUST` existir.
- `entities[].name` `MUST` ser único.
- `operation_effects` `MAY` omitirse si no hay mapeo request->DB confiable.
- Si el schema no está disponible temporalmente, `SHOULD` devolver `503 schema_not_available`.

Uso esperado:

- Permite estimar impacto de requests sobre entidades.
- Permite comparar predicción de impacto con `state.fingerprint` real.

### 7.3 `openapi` (optional fallback)

Discovery:

- Si el servidor anuncia `openapi`, `MUST` exponer al menos una de estas opciones:
  - `links.openapi_url`
  - `GET /_toollab/openapi`

Semántica:

- `openapi` es capability opcional y `SHOULD` usarse como fallback cuando no hay `profile` o cuando se requiere discovery de endpoints no cubiertos por `suggested_flows`.

Endpoint opcional:

```http
GET /_toollab/openapi
```

Contrato:

- `Content-Type` `MUST` ser uno de:
  - `application/json`
  - `application/yaml`
  - `application/vnd.oai.openapi`
- El documento `MUST` ser OpenAPI 3.x válido.
- Si no está disponible temporalmente, `SHOULD` devolver `503 openapi_not_available`.

### 7.4 `suggested_flows`

Endpoint:

```http
GET /_toollab/suggested_flows
```

Contrato:

- Respuesta `MUST` validar contra `schemas/toollab.standard.suggested_flows.v1.1.schema.json`.
- `flows[].id` `MUST` ser único.
- Cada request de flow `MUST` cumplir "exactly one of `body` or `json_body`".
- Placeholders (`{{...}}`) `MAY` usarse en strings y `SHOULD` ser estables/reproducibles.
- Si no está disponible temporalmente, `SHOULD` devolver `503 suggested_flows_not_available`.

### 7.5 `invariants`

Endpoint:

```http
GET /_toollab/invariants
```

Contrato:

- Respuesta `MUST` validar contra `schemas/toollab.standard.invariants.v1.1.schema.json`.
- `invariants[].id` `MUST` ser único.
- Tipos normativos soportados por Toollab v1:
  - `no_5xx_allowed`
  - `max_4xx_rate`
  - `status_code_rate`
  - `idempotent_key_identical_response`
- `type=custom` `MAY` existir como metadata, pero un cliente `MUST NOT` usarlo para PASS/FAIL automático salvo mapeo explícito.

### 7.6 `limits`

Endpoint:

```http
GET /_toollab/limits
```

Contrato:

- Respuesta `MUST` validar contra `schemas/toollab.standard.limits.v1.1.schema.json`.
- Valores numéricos `MUST` ser no negativos.
- El cliente `SHOULD` usar estos límites para acotar `concurrency`, `tick_ms`, `duration_s`.
- El cliente `MAY` seguir ejecutando fuera de límites, pero `SHOULD` dejarlo explícito en evidencia.

### 7.7 `environment`

Endpoint:

```http
GET /_toollab/environment
```

Contrato:

- Respuesta `MUST` validar contra `schemas/toollab.standard.environment.v1.1.schema.json`.
- El payload `MUST` ser informativo (read-only) y `MUST NOT` incluir secretos.
- `mode` `SHOULD` tomar valores conocidos (`dev`, `test`, `staging`, `prod`) o un string custom controlado.

## 8. Determinism requirements for new capabilities

- Para `profile`, `schema`, `suggested_flows`, `invariants`, `limits`, `environment`:
  - Si el estado base no cambió, la respuesta `SHOULD` ser estable entre llamadas.
  - Listas `SHOULD` devolverse en orden estable.
  - Campos opcionales ausentes `SHOULD` omitirse en vez de alternar `null`/missing.
- Un cliente determinista `MUST` canonicalizar estas respuestas antes de hashearlas.

## 9. Size and pagination requirements

- Endpoints potencialmente grandes (`profile`, `logs`, `traces`, `schema`, `openapi`) `SHOULD` soportar límites.
- Recomendación mínima:
  - query `limit` para listas.
  - máximo hard server-side documentado.
  - devolver `413` si el request excede límites configurados.
- `openapi` `SHOULD` soportar compresión HTTP (`gzip`) cuando esté disponible.

## 10. Error codes (recommended)

Tabla mínima recomendada:

| Error | HTTP | Meaning |
|---|---:|---|
| `method_not_allowed` | 405 | Método HTTP inválido para ese endpoint |
| `not_supported` | 501 | Capability no implementada |
| `profile_not_available` | 503 | Profile temporalmente no disponible |
| `schema_not_available` | 503 | Metadata de schema temporalmente no disponible |
| `openapi_not_available` | 503 | OpenAPI temporalmente no disponible |
| `suggested_flows_not_available` | 503 | Flows sugeridos temporalmente no disponibles |
| `invariants_not_available` | 503 | Invariants temporalmente no disponibles |
| `limits_not_available` | 503 | Limits temporalmente no disponibles |
| `environment_not_available` | 503 | Environment temporalmente no disponible |
| `bad_request` | 400 | Parámetros inválidos |
| `internal` | 500 | Error interno |

## 11. Capability matrix

| Capability | Endpoint or link | Schema |
|---|---|---|
| `manifest` | `GET /_toollab/manifest` | `schemas/toollab.standard.manifest.v1.1.schema.json` |
| `profile` | `GET /_toollab/profile` or `links.profile_url` | `schemas/toollab.standard.profile.v1.1.schema.json` |
| `schema` | `GET /_toollab/schema` | `schemas/toollab.standard.schema.v1.1.schema.json` |
| `openapi` | `links.openapi_url` or `GET /_toollab/openapi` | OpenAPI 3.x (fallback) |
| `suggested_flows` | `GET /_toollab/suggested_flows` | `schemas/toollab.standard.suggested_flows.v1.1.schema.json` |
| `invariants` | `GET /_toollab/invariants` | `schemas/toollab.standard.invariants.v1.1.schema.json` |
| `limits` | `GET /_toollab/limits` | `schemas/toollab.standard.limits.v1.1.schema.json` |
| `environment` | `GET /_toollab/environment` | `schemas/toollab.standard.environment.v1.1.schema.json` |

## 12. Adoption levels

| Level | Required set | Outcome |
|---|---|---|
| `L0` | `manifest` | Adapter discoverable |
| `L1` | `L0` + observability core (`metrics` and optional `traces`,`logs`) | Richer evidence |
| `L2` | `L1` + `state.fingerprint` | State-change verification |
| `L3` | `L2` + `state.snapshot`,`state.restore` (+optional `state.reset`) | State reproducibility |
| `L4` | `L3` + `seed` | End-to-end determinism support |
| `L5` | `L4` + `profile` | Single-call discovery baseline for audits |
| `L6` | `L5` + `profile` with `schema` + `suggested_flows` | DB impact prediction + app-driven flows |
| `L7` | `L6` + `profile` with `invariants`,`limits`,`environment` | Policy-aware assertions and contextual evidence |
| `L8` | `L7` + `openapi` fallback | Contract fallback for endpoint discovery gaps |

## 13. Client flow (recommended)

1. `GET /_toollab/manifest`.
2. Leer capabilities y links.
3. Si hay `seed`, aplicar seed antes de correr.
4. Si hay `state.fingerprint`/`metrics`, tomar baseline.
5. Construir escenario con prioridad:
   - `profile` (si existe): usar `suggested_flows`/`invariants`/`limits`/`schema` desde una sola respuesta.
   - Si no hay `profile`, usar endpoints individuales (`suggested_flows`, `invariants`, `limits`, `schema`).
   - `openapi` solo fallback si faltan flows o hay endpoints no cubiertos.
   - fallback final: scenario YAML manual.
6. Ejecutar workload contra endpoints de negocio (no `/_toollab/*`).
7. Recolectar observabilidad final y estado final.
8. Aplicar invariants normativos; registrar `custom` como unknown/documental.
9. Persistir evidencia y hashes deterministas.

## 14. Conformance

Una app es "Toollab Standard v1.1 conformant" si:

- Implementa `manifest` según schema.
- Toda capability anunciada responde con el shape definido por su schema/contrato.
- Respeta formato de errores.
- Cumple restricciones de seguridad y no filtrado de secretos.
- Si anuncia `profile`, este `MUST` ser consistente con los endpoints individuales equivalentes cuando ambos existan.

Formato recomendado de salida de auditoría de conformidad:

- `schemas/toollab.standard.conformance_report.v1.1.schema.json`
- Guía operacional reusable: `docs/STANDARD_CONFORMANCE.md`

Este documento define el contrato extendido para maximizar portabilidad y reproducibilidad entre adapters y clientes Toollab.
