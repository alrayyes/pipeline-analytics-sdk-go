#!/usr/bin/env bash
# Checks lefthook.yml restages through `stage_fixed`, never a hand-written
# `git add`: pre-commit fixers write only the staged files and lefthook puts
# them back, with its own failure handling.
set -euo pipefail

file="${1:-$(dirname "${BASH_SOURCE[0]}")/../lefthook.yml}"

fail() { echo "FAIL: $1" >&2; exit 1; }

if grep -Eq 'git add' "$file"; then
  fail "lefthook.yml restages with a hand-written git add; use stage_fixed: true"
fi
grep -Eq '^[[:space:]]+stage_fixed: true$' "$file" || fail "no pre-commit fixer sets stage_fixed: true"
echo "ok: lefthook.yml restages through stage_fixed"
