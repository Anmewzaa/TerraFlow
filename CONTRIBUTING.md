# Contributing

Thanks for working on TerraFlow. Read [`CLAUDE.md`](CLAUDE.md) and
[`docs/01-scope.md`](docs/01-scope.md) before changing behavior, APIs, the HCL
generator, storage, or the UI. TerraFlow stops at creating a branch on Azure
DevOps; everything after that is DevOps's job in ADO.

## Local setup

- **Backend:** Go (stable). `gofmt`, `go vet`, and ideally `golangci-lint`.
- **Frontend:** Node LTS. `npm --prefix web ci`.
- **Data:** a local PostgreSQL for integration tests. No Azure or ADO access is
  needed to develop the parser/generator — use `samples/`.

Never commit secrets, tokens, real resource names, or prompts. `samples/` is
synthetic and must stay synthetic.

## Common commands

Backend (from repo root):

```bash
gofmt -l .
go vet ./...
go test ./...            # includes HCL golden-file and round-trip tests
```

Frontend:

```bash
npm --prefix web run lint
npm --prefix web run build
npm --prefix web test
```

## Workflow

1. Keep changes focused and within the directory ownership in `CLAUDE.md`.
2. Add or update tests when behavior changes. The HCL generator's golden-file
   and round-trip tests are mandatory and must not be weakened to pass a diff.
3. Update the matching `docs/` file. When a decision changes, add a new `D-xxx`
   entry to `docs/04-decisions.md` — never rewrite an old one.
4. Run the verification subset for what you touched before opening a PR.

## Compatibility-sensitive surfaces

Generated HCL (style/layout), `api/openapi.yaml`, Postgres schema/migrations and
the canvas JSONB shape, the ADO interaction and Workload Identity scope, and the
Entra group-claim → role mapping. Prefer additive changes; add migrations rather
than editing released ones; keep PR diffs on generated HCL minimal.

## Pull requests

Use `.github/PULL_REQUEST_TEMPLATE.md`. Include what changed, why, compatibility
or migration notes, and the checks you ran. Keep credentials, real names and
prompts out of descriptions, logs, screenshots and fixtures.
