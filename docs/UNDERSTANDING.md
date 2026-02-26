# Understanding Layer (map / explain / diff)

This document defines the human-comprehension layer for TOOLAB v1.

## Principles

- Evidence-driven only: no claim without support in `evidence.json` or discovery payloads.
- Deterministic by default: same inputs produce byte-identical `system_map.json`, `understanding.json`, and `diff.json`.
- No impact on assertions PASS/FAIL: understanding is narrative and audit-focused.

## Commands

### `toolab map`

Builds a system map from discovery:

```bash
toolab map --from openapi --openapi-file ./openapi.yaml --out ./artifacts
toolab map --from toolab --target-base-url http://localhost:8080 --out ./artifacts
```

Outputs:

- `system_map.json`
- `system_map.md`
- `map.meta.json`

### `toolab explain`

Explains a completed run:

```bash
toolab explain ./golden_runs/<run_id> --out ./artifacts
```

Outputs:

- `understanding.json`
- `understanding.md`
- `explain.meta.json`

### `toolab diff`

Compares two runs:

```bash
toolab diff ./golden_runs/<run_a> ./golden_runs/<run_b> --out ./artifacts
```

Outputs:

- `diff.json`
- `diff.md`
- `diff.meta.json`

## Unknowns and no-claim policy

- If discovery is unavailable, TOOLAB builds partial understanding and records `unknowns`.
- Unsupported or missing signals are reported explicitly, not inferred.
- LLM mode (`--llm on`) is narrative-only and cannot alter PASS/FAIL.
