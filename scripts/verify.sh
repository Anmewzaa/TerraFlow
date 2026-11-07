#!/usr/bin/env bash
# Run the verification subset for what exists. Fail-fast.
set -euo pipefail
cd "$(dirname "$0")/.."

if command -v go >/dev/null 2>&1 && [ -f go.mod ]; then
  echo "== go =="
  gofmt -l . | (grep . && { echo "gofmt: files need formatting"; exit 1; } || true)
  go vet ./...
  go test ./...
else
  echo "== go: toolchain or go.mod not found, skipping =="
fi

if command -v npm >/dev/null 2>&1 && [ -f web/package.json ]; then
  echo "== web =="
  npm --prefix web run lint
  npm --prefix web run typecheck
  npm --prefix web test
  npm --prefix web run build
else
  echo "== web: npm or web/package.json not found, skipping =="
fi
echo "verify: done"
