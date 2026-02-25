# Decisions Log

## D-001: Runtime language

- Status: accepted
- Decision: implement ToolLab core in Go 1.22+
- Rationale: single static binary, predictable concurrency model, strong test tooling

## D-002: V1 chaos error mode

- Status: accepted
- Decision: `chaos.error_mode` is fixed to `abort` in v1
- Rationale: deterministic client-side fault injection without synthetic upstream responses

## D-003: Deterministic scheduling

- Status: accepted
- Decision:
  - `open_loop`: exactly one planned request per tick
  - `closed_loop`: deterministic pre-planned stream independent of goroutine completion order
- Rationale: reproducibility under concurrency

## D-004: Deterministic hashing subset

- Status: accepted
- Decision: timestamps/environment/observability are excluded from deterministic fingerprint
- Rationale: avoid non-deterministic noise in reproducibility checks

## D-005: Body contract in scenario requests

- Status: accepted
- Decision: request must contain exactly one of `body` or `json_body`
- Rationale: remove ambiguity and force explicit payload semantics in v1
