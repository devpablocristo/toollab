# TOOLAB Understanding

## 1) Que es este servicio
Service identified as unknown-service (version 0.1.0).

## 2) Como se usa
Suggested flows: flow_001

## 3) Que se probo
Scenario 59790cc1a9b15d1ac2faa9ffcb002b1bc8ed3050ef5a153ac7d9f8573fc81647 executed with 10 planned requests and 10 completed.

## 4) Que paso
Observed error_rate=1.0000, p50=0ms, p95=0ms, p99=0ms.

## 5) Que fallo
Violated rules: threshold_error_rate

## 6) Que esta probado
Assertions did not fully pass.

## 7) Que es unknown
unknowns: discovery unavailable: map derived from observed outcomes only; logs_excerpt unavailable: not configured; metrics_snapshot unavailable: not configured; trace_refs unavailable: not configured

## 8) Como reproducir
Reproduce with: toolab run ../testdata/e2e/scenario.yaml --out ../golden_runs (expected fingerprint 5deef5b7c84e8a65c66837f5f10525c4f0d71b62a42291476daccfbf51c68d98).

## Claims
- [SUPPORTED] Assertion overall result is FAIL.
- [SUPPORTED] P95 latency = 0ms.
- [UNKNOWN] Some claims cannot be supported with available evidence.
  - missing: discovery unavailable: map derived from observed outcomes only, logs_excerpt unavailable: not configured, metrics_snapshot unavailable: not configured, trace_refs unavailable: not configured
- [SUPPORTED] Total requests = 10.

## Anchors
- json_pointer:/assertions/overall
- json_pointer:/stats/p95_ms
- json_pointer:/stats/total_requests
- json_pointer:/unknowns
