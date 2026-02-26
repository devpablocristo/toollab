#!/usr/bin/env bash
set -euo pipefail

SCENARIO_PATH="${1:-../testdata/e2e/scenario.yaml}"
OUT_BASE="${2:-./golden_runs}"
EXPECTED="5deef5b7c84e8a65c66837f5f10525c4f0d71b62a42291476daccfbf51c68d98"

toollab run "$SCENARIO_PATH" --out "$OUT_BASE"
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
