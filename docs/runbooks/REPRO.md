# Repro Runbook

## Goal

Reproduce a previous run and verify deterministic fingerprint.

## Steps

1. Locate run artifacts under `golden_runs/<run_id>/`.
2. Execute:

```bash
bash golden_runs/<run_id>/repro.sh <scenario-path> <out-base>
```

3. Script compares `actual` vs `expected` deterministic fingerprint.

## Troubleshooting

- If fingerprint differs, verify:
  - same scenario content
  - same seeds
  - deterministic mock/SUT behavior
  - same Toolab decision engine version
