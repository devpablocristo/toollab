# Decisions Log

## D-001 Runtime

- Status: accepted
- Decision: core runtime implemented in Go 1.22+.

## D-002 Naming

- Status: accepted
- Decision: product and CLI naming is `toolab` (single `l`).

## D-003 Chaos error mode v1

- Status: accepted
- Decision: `chaos.error_mode` fixed to `abort` in v1.

## D-004 Scheduling determinism

- Status: accepted
- Decision:
  - `open_loop` plans one request per tick.
  - `closed_loop` uses deterministic pre-planned stream independent of goroutine completion order.

## D-005 Fingerprint subset

- Status: accepted
- Decision: deterministic fingerprint excludes informational fields (`timestamps`, `environment`, `observability`).

## D-006 Body contract

- Status: accepted
- Decision: each request must include exactly one of `body` or `json_body`.
