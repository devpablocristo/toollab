# Toolab

Toolab v1 (black mode) is a deterministic behavior lab for HTTP APIs.

## Purpose

Toolab executes reproducible HTTP scenarios against black-box systems, injects deterministic client-side chaos, captures structured evidence, evaluates deterministic assertions, and generates reproducible reports.

## Key Guarantees

- `run_seed` and `chaos_seed` are mandatory.
- Decision-making is deterministic by stable key:
  `(seed, stream, request_seq, decision_type, decision_idx, key_extra...)`.
- Concurrency does not alter deterministic decisions.
- Outcomes are sorted by `seq` before aggregation.
- Percentiles use nearest-rank over integer milliseconds.
- PASS/FAIL is deterministic and does not use LLM.

## Repository Layout

- `docs/`: specs, determinism contract, decisions, runbooks
- `schemas/`: JSON Schema contracts for scenario/evidence
- `testdata/`: valid/invalid fixtures + e2e scenario
- `toolab-core/`: Go CLI/runtime implementation

## CLI

```bash
toolab run scenario.yaml
```

With output base directory:

```bash
toolab run scenario.yaml --out golden_runs
```

Generation and enrichment:

```bash
toolab generate --from openapi --openapi-file ./openapi.yaml --out ./scenario.generated.yaml
toolab enrich ./scenario.generated.yaml --from toolab --target-base-url http://localhost:8080 --out ./scenario.enriched.yaml
```

Understanding layer:

```bash
toolab map --from toolab --target-base-url http://localhost:8080 --out ./artifacts
toolab explain ./golden_runs/<run_id> --out ./artifacts
toolab diff ./golden_runs/<run_a> ./golden_runs/<run_b> --out ./artifacts
```

## Build and Test

```bash
make test
```

## Outputs per Run

Run directory: `golden_runs/<run_id>/`

- `evidence.json`
- `report.json`
- `report.md`
- `junit.xml`
- `decision_tape.jsonl`
- `repro.sh`
- `system_map.json`
- `system_map.md`
- `understanding.json`
- `understanding.md`

## Reproducibility

Same scenario + same seeds + stable SUT/mock produces identical:

- `decision_tape_hash`
- `deterministic_fingerprint`

On live mutable systems, `decision_tape_hash` is the primary determinism signal.
`deterministic_fingerprint` can vary if SUT behavior/state changes between runs.

Use generated script:

```bash
bash golden_runs/<run_id>/repro.sh <scenario-path> <out-base>
```

The script checks expected deterministic fingerprint.
