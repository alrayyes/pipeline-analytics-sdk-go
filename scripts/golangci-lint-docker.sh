#!/usr/bin/env bash
# Runs golangci-lint inside its own pinned image (rules/go-lint.md), which
# bundles a matched go build so the version it type-checks with is never a
# question the host's package manager gets a vote on.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

LINT_IMAGE="golangci/golangci-lint:v2.13.2"

# See scripts/go-docker.sh for why these are pre-created rather than left
# for Docker to auto-create (root-owned) on a fresh machine.
mkdir -p "${HOME}/.cache/go-build-docker" "${HOME}/.cache/golangci-lint-docker"

docker run --rm --user "$(id -u):$(id -g)" \
  -v "$(pwd):/src" -w /src \
  -e HOME=/tmp \
  -e GOCACHE=/gocache -v "${HOME}/.cache/go-build-docker:/gocache" \
  -e GOLANGCI_LINT_CACHE=/cache -v "${HOME}/.cache/golangci-lint-docker:/cache" \
  "$LINT_IMAGE" golangci-lint "$@"
