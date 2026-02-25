# Determinism Contract (V1)

## Mandatory constraints

1. Seeds are mandatory (`run_seed`, `chaos_seed`); missing seed is a hard error.
2. PRNG engine is versioned: `splitmix64+xoshiro256ss-v1`.
3. Decision key is stable and explicit:
   `(seed, stream, request_seq, decision_type, decision_idx, key_extra...)`.
4. Concurrency cannot alter decision outcomes.
5. Outcomes are sorted by `seq` before evidence aggregation.
6. Percentiles are nearest-rank on integer milliseconds.
7. Assertions use only deterministic evidence data.

## Hashes

- `scenario_sha256`: canonical scenario hash
- `decision_tape_hash`: hash of sorted decision tape entries
- `run_id`: sha256 of `scenario_sha256`, seeds, and decision engine version
- `deterministic_fingerprint`: hash of deterministic evidence subset
