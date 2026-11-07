---
name: change-verification
description: Run the right TerraFlow verification before completing a change — Go, frontend, OpenAPI drift, and DB migrations, scoped to what was touched. Use before opening a PR for code/test/schema/build changes.
---

# Change verification

Run before marking code work complete. Fail-fast: fix and rerun until the
required checks for the touched areas pass. Skip only for docs-only or
metadata-only changes (say so in the PR).

## Backend — `server/` touched
```bash
gofmt -l .            # must print nothing
go vet ./...
go test ./...         # includes HCL golden-file + round-trip suites
golangci-lint run     # if configured
```
Generator/parser changes **must** keep the round-trip byte-stable for untouched
content and only change golden files you intended. Never weaken these tests to
make a diff pass.

## Frontend — `web/` touched
```bash
npm --prefix web ci
npm --prefix web run lint
npm --prefix web run build     # tsc + vite
npm --prefix web test          # vitest
```

## Contract — `api/openapi.yaml` touched
Regenerate the Go server and TS client and confirm no drift (regen, then
`git diff --exit-code` on the generated files).

## Database — migrations touched
Apply then roll back on a throwaway Postgres:
```bash
goose up && goose down
```
Add backward-read coverage if the canvas JSONB shape changed.

## If the stack doesn't exist yet
Early in the project a tool/dir may be missing. Record which checks were N/A
(e.g. "no `web/` yet") — don't treat absence as a pass once the code lands.
