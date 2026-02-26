# Toolab Report

## 1. Executive summary (30s)
- overall: **PASS**
- total_requests: 40
- error_rate: 0.025000
- p95_ms: 50
- deterministic_fingerprint: `316aaba15faf3f431ba5ebc85ee2068eeffffaa3e43daf29b8bf6e54c67e7d8e`

## 2. Qué pasó
- success_rate: 0.975000
- p50/p95/p99: 25/50/51 ms
- status_histogram: `map[200:39]`

## 3. Qué se rompió
- no violated rules

## 4. Qué está probado
- threshold_error_rate
- threshold_p95_ms
- invariant_00_no_5xx_allowed
- invariant_01_max_4xx_rate

## 5. Qué es unknown
- metrics_snapshot unavailable: not configured
- trace_refs unavailable: not configured
- logs_excerpt unavailable: not configured

## 6. Cómo reproducir
- command: `toolab run ../scenarios/nexus_health.yaml --out ../golden_runs`
- script: ``
- expected fingerprint: `316aaba15faf3f431ba5ebc85ee2068eeffffaa3e43daf29b8bf6e54c67e7d8e`
