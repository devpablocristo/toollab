# Generation Command (toollab generate)

This document defines the normative behavior for scenario generation in `toollab`.

## Scope

- `toollab generate` creates a Scenario v1 YAML without changing `toollab run` semantics.
- `toollab gen` is a backward-compatible alias of:
  - `toollab generate --from openapi`

## Command forms

```bash
toollab generate --from openapi --openapi-file <path> --out <scenario.yml> [flags]
toollab generate --from openapi --openapi-url <url>   --out <scenario.yml> [flags]

toollab generate --from toollab --target-base-url <url> --out <scenario.yml> [flags]
```

## Required/optional flags

- Required:
  - `--from openapi|toollab`
  - `--out <file>` unless `--print` is set
- Optional:
  - `--seed <decimal_string>`
  - `--mode smoke|load|chaos` (default `smoke`)
  - `--base-url <url>` (OpenAPI override)
  - `--toollab-url <url>` (default `<target-base-url>/_toollab`)
  - `--prefer profile|endpoints` (default `profile`)
  - `--flow-source suggested_flows|openapi_fallback|manual` (default `suggested_flows`)
  - `--require-capability <cap>` (repeatable)
  - `--print`
  - `--dry-run`

## Deterministic seed

- If `--seed` is provided:
  - `effective_seed = --seed`
- If `--seed` is not provided:
  - `inputs_canonical = canonical_json({inputs, options})`
  - `seed_bytes = sha256(inputs_canonical)[0:8]`
  - `effective_seed = uint64_from_be_bytes(seed_bytes)`
  - serialize as decimal string

### Normative input hashing

- File input:
  - `sha256(file_bytes)`
- URL input:
  - `sha256(response_bytes)` + canonical URL
  - canonical URL: lowercase host, no trailing slash, sorted query params
- TOOLLAB endpoint responses:
  - hash `canonical_json(response_body)`
- Include all options used by generation.
- Exclude `generated_at_utc`.

## Generation levels

### Level 1: `--from openapi`

- Source: OpenAPI file/URL.
- Endpoint selection:
  - exclude `/_toollab/*`
  - prefer GET in `smoke`
  - include mutations when examples or clear schemas exist
- Request construction:
  - `request_fingerprint = METHOD + normalized_path + canonical_query_kv + content_type + body_shape_hash`
  - `id` base: `<method>_<path_sanitized>`
  - collisions: suffix `_<hash6(request_fingerprint)>`
  - `content_type` default `application/json` when body exists
  - body preference: `example` first, then minimal deterministic payload from schema

### Level 2: `--from toollab`

- Source: TOOLLAB Standard `/_toollab/*`.
- Discovery flow:
  1. `GET /_toollab/manifest`
  2. if `profile` exists and `--prefer profile`, use `GET /_toollab/profile`
  3. otherwise use declared endpoints individually
- Scenario composition priority:
  - requests from `suggested_flows`
  - invariants from normative types only
  - limits used for defaults
  - observability inferred from capabilities
- Fallback:
  - if missing `suggested_flows` and OpenAPI exists, use OpenAPI fallback and add warning

## Output behavior

- YAML output must be Scenario v1 valid and canonical.
- `generate.meta.json` must always be produced:
  - write file alongside `--out` by default
  - print to stdout in `--print` and `--dry-run` modes
- `--print`:
  - print canonical scenario YAML to stdout
- `--dry-run`:
  - do not write scenario file or meta file
  - still compute full generation and print meta

## Security

- Never persist secrets in scenario or meta.
- Auth values are passed via env var references only.
- Discovery clients must enforce:
  - timeout
  - hard byte limits
  - optional gzip handling
  - redaction-safe logging
