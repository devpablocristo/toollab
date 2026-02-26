# Toollab Scenario Spec (V1)

Canonical schema: `schemas/scenario.v1.schema.json`.

## Required top-level fields

- `version: 1`
- `mode: black`
- `target`
- `workload`
- `chaos`
- `expectations`
- `seeds`

## Determinism-critical rules

- `seeds.run_seed` and `seeds.chaos_seed` are mandatory.
- `request_seq` is pre-assigned by planner.
- Request selection is deterministic by `(run_seed, stream=workload_pick, request_seq)`.
- Chaos decisions are deterministic by `(chaos_seed, stream, request_seq, decision_type, decision_idx, ...)`.

## Scheduling rules

- `open_loop`: `num_ticks = floor(duration_s*1000 / tick_ms)` and exactly one planned request per tick.
- `closed_loop`: deterministic pre-planned stream (`duration_s` and `concurrency`) independent from goroutine completion order.

## Request payload contract

- Exactly one of `body` or `json_body` must be present.

## Canonicalization and fingerprint

Scenario is normalized (defaults applied), serialized to canonical JSON, then hashed to produce `scenario_sha256`.
