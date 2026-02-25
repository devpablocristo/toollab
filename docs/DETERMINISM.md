# Determinism Contract (V1)

## Non-negotiables

1. `run_seed` and `chaos_seed` are mandatory. Missing seed is a hard error.
2. Pseudo-random decisions are derived only from stable decision keys.
3. Concurrency cannot influence decision outcomes.
4. Percentiles are nearest-rank over integer milliseconds.
5. PASS/FAIL uses deterministic evidence fields only.

## Decision Key

All pseudo-random choices use:

`(seed, stream, request_seq, decision_type, decision_idx, key_extra...)`

## Ordering Rules

1. `request_seq` is assigned before execution.
2. Outcomes are sorted by `seq` before aggregation/persistence.
3. Deterministic sampling is based on `seq` and seed.

## Hashes

- `scenario_sha256`: canonical scenario hash
- `decision_tape_hash`: hash of deterministic decision tape
- `deterministic_fingerprint`: hash of deterministic evidence subset
