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

## Reproducibility

Same scenario + same seeds + stable SUT/mock produces identical:

- `decision_tape_hash`
- `deterministic_fingerprint`

Use generated script:

```bash
bash golden_runs/<run_id>/repro.sh <scenario-path> <out-base>
```

The script checks expected deterministic fingerprint.
