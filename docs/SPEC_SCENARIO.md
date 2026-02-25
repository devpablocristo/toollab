# ToolLab Scenario Spec (V1)

Authoritative schema: `schemas/scenario.v1.schema.json`.

## Required top-level fields

- `version` (must be `1`)
- `mode` (must be `black`)
- `target`
- `workload`
- `chaos`
- `expectations`
- `seeds`

## Determinism-critical fields

- `seeds.run_seed`
- `seeds.chaos_seed`
- `workload.schedule_mode`
- `workload.tick_ms` (required for open loop)

## Canonicalization

Scenario is normalized with defaults and serialized to canonical JSON for `scenario_sha256`.
