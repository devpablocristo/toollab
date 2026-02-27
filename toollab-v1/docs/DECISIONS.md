# Decisions Log

## D-001 Runtime

- Status: accepted
- Decision: core runtime implemented in Go 1.22+.

## D-002 Naming

- Status: accepted
- Decision: product and CLI naming is `toollab` (single `l`).

## D-003 Chaos error mode v1

- Status: accepted
- Decision: `chaos.error_mode` fixed to `abort` in v1.

## D-004 Scheduling determinism

- Status: accepted
- Decision:
  - `open_loop` plans one request per tick.
  - `closed_loop` uses deterministic pre-planned stream independent of goroutine completion order.

## D-005 Fingerprint subset

- Status: accepted
- Decision: deterministic fingerprint excludes informational fields (`timestamps`, `environment`, `observability`).

## D-006 Body contract

- Status: accepted
- Decision: each request must include exactly one of `body` or `json_body`.

## D-007 Adapter profile aggregation

- Status: accepted
- Decision:
  - `profile` is the primary adapter discovery capability for Toollab audits.
  - `openapi` remains optional and is used as fallback when profile/flows are insufficient.

## D-008 API-agnostic standard packaging

- Status: accepted
- Decision:
  - Toollab Standard is domain-agnostic and reusable for any HTTP API.
  - Conformance output is standardized through a reusable report schema and checklist.

## D-009 Generate command alias

- Status: accepted
- Decision:
  - `toollab gen` remains backward-compatible.
  - `toollab gen` is an alias of `toollab generate --from openapi`.

## D-010 Derived seed algorithm

- Status: accepted
- Decision:
  - When `--seed` is absent in `generate` and `enrich`, derive deterministic seed from canonical inputs.
  - Derivation:
    - `inputs_canonical = canonical_json({inputs, options})`
    - `seed_bytes = sha256(inputs_canonical)[0:8]`
    - `effective_seed = uint64_from_be_bytes(seed_bytes)` (big-endian), serialized as decimal string.
  - Input hashing rules:
    - file inputs: `sha256(file_bytes)`
    - URL inputs: `sha256(response_bytes)` plus canonical URL
    - TOOLLAB endpoint payloads: hash `canonical_json(response_body)`
  - `generated_at_utc` is excluded from seed derivation.

## D-011 Output path policy

- Status: accepted
- Decision:
  - `--out` is mandatory for `generate` and `enrich` unless `--print` is set.
  - No implicit overwrite destinations are allowed.

## D-012 Profile precedence and OpenAPI fallback

- Status: accepted
- Decision:
  - For TOOLLAB discovery, `/_toollab/profile` is the primary source when available.
  - OpenAPI is fallback only when profile/flows are insufficient.

## D-013 Naming policy

- Status: accepted
- Decision:
  - Product, CLI, and schemas use `toollab` (single `l`) only.
  - The string `toollab` is prohibited in paths, schema ids, and public fields.

## D-014 Enrichment changes path format

- Status: accepted
- Decision:
  - `changes[].path` in meta must use JSON Pointer (RFC 6901).

## D-015 Request fingerprint key

- Status: accepted
- Decision:
  - `request_fingerprint` must include `content_type`:
    - `METHOD + normalized_path + canonical_query_kv + content_type + body_shape_hash`

## D-016 Numeric formatting in canonical writers

- Status: accepted
- Decision:
  - Keep integers as integers whenever possible.
  - Avoid floating-point emission when not required by schema.
  - When floats are required, emit using stable formatting (`strconv.FormatFloat(..., 'f', -1, 64)` equivalent behavior) in canonical writers.

## D-017 Understanding no-claim policy

- Status: accepted
- Decision:
  - `map/explain/diff` are evidence-driven.
  - Claims without enough evidence must be emitted as `unknown`.
  - Understanding artifacts never modify deterministic PASS/FAIL assertions.

## D-018 Deterministic understanding artifacts

- Status: accepted
- Decision:
  - `system_map.json`, `understanding.json`, and `diff.json` omit runtime timestamps by default.
  - Fingerprints are computed from canonical JSON excluding informational fields.

## D-019 Enrich aggressive precedence

- Status: accepted
- Decision:
  - In `aggressive` mode, TOOLLAB discovery may overwrite non-critical defaults.
  - OpenAPI discovery never overwrites existing manual/TOOLLAB values; it only fills gaps.

## D-020 Run CLI argument order

- Status: accepted
- Decision:
  - `toollab run` accepts `--out` before or after scenario path.
  - Parsing is deterministic and rejects unknown flags.
