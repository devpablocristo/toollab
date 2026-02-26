# Toollab Report

## 1. Executive summary (30s)
- overall: **FAIL**
- total_requests: 10
- error_rate: 1.000000
- p95_ms: 0
- deterministic_fingerprint: `5deef5b7c84e8a65c66837f5f10525c4f0d71b62a42291476daccfbf51c68d98`

## 2. Qué pasó
- success_rate: 0.000000
- p50/p95/p99: 0/0/0 ms
- status_histogram: `map[404:10]`

## 3. Qué se rompió
- threshold_error_rate

## 4. Qué está probado
- threshold_p95_ms
- invariant_00_no_5xx_allowed

## 5. Qué es unknown
- metrics_snapshot unavailable: not configured
- trace_refs unavailable: not configured
- logs_excerpt unavailable: not configured

## 6. Cómo reproducir
- command: `toollab run ../testdata/e2e/scenario.yaml --out ../golden_runs`
- script: ``
- expected fingerprint: `5deef5b7c84e8a65c66837f5f10525c4f0d71b62a42291476daccfbf51c68d98`
