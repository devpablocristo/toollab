# ToolLab Evidence Spec (V1)

Authoritative schema: `schemas/evidence.v1.schema.json`.

## Required deterministic anchors

- `metadata.run_seed`
- `metadata.chaos_seed`
- `scenario_fingerprint.scenario_sha256`
- `execution.decision_engine_version`
- `execution.decision_tape_hash`
- `deterministic_fingerprint`

## Deterministic fingerprint

Computed over a canonical subset of evidence excluding non-deterministic informational fields.

## Redaction policy

Samples are redacted before persistence.

Default sensitive headers:

- `authorization`
- `cookie`
- `set-cookie`
- `x-api-key`
