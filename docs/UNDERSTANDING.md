# Understanding Layer (map / explain / diff)

This document defines the human-comprehension layer for TOOLLAB v1.

## Principles

- Evidence-driven only: no claim without support in `evidence.json` or discovery payloads.
- Deterministic by default: same inputs produce byte-identical `system_map.json`, `understanding.json`, and `diff.json`.
- No impact on assertions PASS/FAIL: understanding is narrative and audit-focused.

## Commands

### `toollab map`

Builds a system map from discovery:

```bash
toollab map --from openapi --openapi-file ./openapi.yaml --out ./artifacts
toollab map --from toollab --target-base-url http://localhost:8080 --out ./artifacts
```

Outputs:

- `system_map.json`
- `system_map.md`
- `map.meta.json`

### `toollab explain`

Explains a completed run:

```bash
toollab explain ./golden_runs/<run_id> --out ./artifacts
```

Outputs:

- `understanding.json`
- `understanding.md`
- `explain.meta.json`

### `toollab diff`

Compares two runs:

```bash
toollab diff ./golden_runs/<run_a> ./golden_runs/<run_b> --out ./artifacts
```

Outputs:

- `diff.json`
- `diff.md`
- `diff.meta.json`

## Unknowns and no-claim policy

- If discovery is unavailable, TOOLLAB builds partial understanding and records `unknowns`.
- Unsupported or missing signals are reported explicitly, not inferred.
- LLM mode (`--llm on`) is narrative-only and cannot alter PASS/FAIL.
