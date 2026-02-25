# Toolab Evidence Spec (V1)

Canonical schema: `schemas/evidence.v1.schema.json`.

## Required anchors

- `metadata.run_seed`
- `metadata.chaos_seed`
- `scenario_fingerprint.scenario_sha256`
- `execution.decision_engine_version`
- `execution.decision_tape_hash`
- `deterministic_fingerprint`

## Deterministic fingerprint

`deterministic_fingerprint` is SHA256 over a canonical deterministic subset of evidence.

Included subset includes:

- scenario fingerprint
- execution (including decision tape hash)
- stats
- outcomes
- samples (already redacted)
- assertions and violated rules
- unknowns
- redaction summary
- repro command

Excluded fields include informational non-deterministic data:

- timestamps
- `environment`
- `observability`

## Redaction

Redaction is applied before sample persistence.

Default sensitive headers:

- `authorization`
- `cookie`
- `set-cookie`
- `x-api-key`
