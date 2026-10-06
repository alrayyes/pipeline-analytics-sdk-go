#!/usr/bin/env bash
# Checks scripts/build-reports.sh turns coverage.out and junit.xml into the
# layout apis.ryankes.eu serves. Run from the repo root after `go test`
# wrote both inputs.
set -euo pipefail

out="$(mktemp -d)"
trap 'rm -rf "$out"' EXIT

./scripts/build-reports.sh "$out"

fail() { echo "FAIL: $1" >&2; exit 1; }

for f in index.html tests/junit.xml coverage/index.html coverage/coverage.xml coverage/coverage.out; do
  [ -s "$out/reports/$f" ] || fail "missing reports/$f"
done
grep -q '<coverage ' "$out/reports/coverage/coverage.xml" || fail "coverage.xml is not Cobertura"
grep -q '<testsuite' "$out/reports/tests/junit.xml" || fail "junit.xml has no testsuite"
grep -q 'coverage/' "$out/reports/index.html" || fail "index.html does not link coverage"
grep -q 'tests/' "$out/reports/index.html" || fail "index.html does not link tests"
echo "ok"
