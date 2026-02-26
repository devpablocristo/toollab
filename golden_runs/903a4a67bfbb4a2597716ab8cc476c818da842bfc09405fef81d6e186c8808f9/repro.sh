#!/usr/bin/env bash
set -euo pipefail

SCENARIO_PATH="${1:-../scenarios/nexus_health.yaml}"
OUT_BASE="${2:-./golden_runs}"
EXPECTED="316aaba15faf3f431ba5ebc85ee2068eeffffaa3e43daf29b8bf6e54c67e7d8e"

toolab run "$SCENARIO_PATH" --out "$OUT_BASE"
LATEST_DIR="$(ls -1dt "$OUT_BASE"/* | head -n 1)"
ACTUAL="$(python3 - <<'PY' "$LATEST_DIR/evidence.json"
import json,sys
print(json.load(open(sys.argv[1]))['deterministic_fingerprint'])
PY
)"

echo "expected: $EXPECTED"
echo "actual:   $ACTUAL"

if [[ "$ACTUAL" != "$EXPECTED" ]]; then
  echo "fingerprint mismatch"
  exit 1
fi

echo "repro ok"
