#!/usr/bin/env bash
# Runs a Go command inside the pinned golang image (rules/go-mod.md) so the
# toolchain version is the one this repo's go.mod says it wants, not
# whatever the host package manager happens to have.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

GO_IMAGE="golang:1.27.1-bookworm"

docker run --rm --user "$(id -u):$(id -g)" \
  -v "$(pwd):/src" -w /src \
  -e HOME=/tmp \
  -e GOCACHE=/gocache -v "${HOME}/.cache/go-build-docker:/gocache" \
  -e GOMODCACHE=/gomod -v "${HOME}/.cache/go-mod-docker:/gomod" \
  "$GO_IMAGE" "$@"
