#!/usr/bin/env bash
# Runs a Go command inside the pinned golang image (rules/go-mod.md) so the
# toolchain version is the one this repo's go.mod says it wants, not
# whatever the host package manager happens to have.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

# One patch ahead of go.mod's 1.27.1 on purpose: 1.27.2 carries the
# standard-library fixes govulncheck reports, but golangci-lint (built with
# 1.27.0) can't read 1.27.2's export data, so go.mod stays put until a
# release is built with 1.27.2 or later.
GO_IMAGE="golang:1.27.2-bookworm"

# Pre-create the cache dirs as the invoking user. Docker auto-creates a
# missing bind-mount source itself (as root, via the daemon) the first
# time a fresh machine -- a CI runner, say -- hits this script, and a
# root-owned mount is one --user can't write into: "mkdir: permission
# denied" on the very first cold run, every time.
mkdir -p "${HOME}/.cache/go-build-docker" "${HOME}/.cache/go-mod-docker"

docker run --rm --user "$(id -u):$(id -g)" \
  -v "$(pwd):/src" -w /src \
  -e HOME=/tmp \
  -e GOCACHE=/gocache -v "${HOME}/.cache/go-build-docker:/gocache" \
  -e GOMODCACHE=/gomod -v "${HOME}/.cache/go-mod-docker:/gomod" \
  "$GO_IMAGE" "$@"
