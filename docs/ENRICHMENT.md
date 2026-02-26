# Enrichment Command (toolab enrich)

This document defines the normative behavior for scenario enrichment in `toolab`.

## Scope

- `toolab enrich` starts from an existing Scenario v1 YAML and enriches it using discovery data.
- `toolab run` behavior remains unchanged.

## Command form

```bash
toolab enrich <scenario.yml> --out <scenario.enriched.yml> [flags]
```

## Flags

- Required:
  - positional `<scenario.yml>`
  - `--out <file>` unless `--print` is set
- Optional:
  - `--from openapi` with one of:
    - `--openapi-file <path>`
    - `--openapi-url <url>`
  - `--from toolab` with:
    - `--target-base-url <url>`
    - optional `--toolab-url <url>`
  - `--seed <decimal_string>`
  - `--merge-strategy conservative|aggressive` (default `conservative`)
  - `--print`
  - `--dry-run`

## Determinism

- If `--seed` is missing, derive seed with the same normative algorithm as `generate`.
- Same input + same options + same seed must produce byte-identical output.
- Enrichment must use stable precedence, stable sort, and canonical writer.

## Merge strategies

### `conservative` (default)

1. Manual scenario values win.
2. TOOLAB Standard fills gaps.
3. OpenAPI fills remaining gaps.

### `aggressive`

- TOOLAB may overwrite only non-critical defaults:
  - `timeout_ms`
  - `weight`
  - `tick_ms`
  - `concurrency`
  - only when current values are defaults
- OpenAPI never overwrites manual or TOOLAB-defined values.

## Gap definition

- Missing field.
- Empty field.
- `timeout_ms == 0`.
- Missing `json_body` where deterministic body can be inferred.

## Request identity, dedupe, ordering

- Dedupe key:
  - `request_fingerprint = METHOD + normalized_path + canonical_query_kv + content_type + body_shape_hash`
- ID collision strategy:
  - base `<method>_<path_sanitized>`
  - suffix `_<hash6(request_fingerprint)>` on collision
- Final order:
  - sort by `request_fingerprint`, then by `id`

## Changeset requirements

- Enrichment must emit deterministic `changes[]` in `generate.meta.json`.
- Each change must include:
  - `op`
  - `path` as JSON Pointer (RFC 6901)
  - `reason`
  - `source` (`manual|toolab|openapi`)
  - `before_hash`
  - `after_hash`

## Output behavior

- Output scenario must be canonical YAML.
- `generate.meta.json` must always be emitted or printed.
- `--print`: print canonical scenario YAML.
- `--dry-run`: do not write files; still compute and print deterministic meta.

## Security

- Do not persist secrets in scenario, meta, or logs.
- Auth for discovery must use env var references only.
- Warnings must avoid leaking token values.
