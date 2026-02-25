# ToolLab

ToolLab v1 (black mode) is a deterministic behavior lab for HTTP APIs.

## Scope

- Reproducible scenarios via YAML DSL
- Deterministic client-side chaos
- Structured evidence bundle
- Deterministic assertions and invariants
- Reproducible reporting artifacts

## Repository Layout

- `docs/`: specs, determinism policy, architecture decisions
- `schemas/`: JSON Schemas for scenario and evidence
- `testdata/`: schema fixtures and integration scenarios
- `toollab-core/`: Go CLI and runtime implementation

## Current Status

Scaffold and contracts are in place. Implementation proceeds in phased commits.
