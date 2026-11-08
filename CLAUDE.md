# CLAUDE.md

Claude Code reads this file automatically. It is the **single guide** for working
on TerraFlow — scope, hard rules, layout, verification and PR process.

TerraFlow is a visual, drag-and-drop portal for Azure infrastructure. Developers
compose resources from an approved module catalog; a DevOps reviewer approves;
TerraFlow then generates Terraform **module calls** and opens a **branch** on the
project's Azure DevOps (ADO) repo. **The app's job ends at the branch** — PR,
pipeline, plan and apply are done by DevOps in ADO.

**Read before changing anything:** `docs/01-scope.md`, `docs/02-tech-stack.md`,
`docs/04-decisions.md`, and `samples/README.md`. For UI work also read
`docs/05-design-system.md` and `web/src/design-system/README.md`. Conventions are
in `docs/06-conventions.md`; the project's vocabulary in `docs/07-glossary.md`.

> Status: design/early build. Some directories below land as their phase does
> (`docs/03-phases.md`). Create them when you reach that phase; don't scaffold
> empty shells ahead of need.

## Start here — read only what your task needs

Don't read every doc. Match your task, read those files, skip the rest.

| Task | Read |
|---|---|
| Anything (always) | this file |
| HCL parser / inventory / graph | `docs/09-graph-contract.md`, `samples/README.md`, `server/internal/hcl/` |
| HCL generator (module calls) | `docs/09-graph-contract.md`, `samples/README.md` (style), `docs/06-conventions.md` |
| API / endpoints | `api/openapi.yaml`, `docs/02-tech-stack.md` |
| Auth / roles / Entra | `docs/02-tech-stack.md` (Security), `docs/01-scope.md` (roles) |
| DB / migrations | `docs/02-tech-stack.md` (Backend), `docs/06-conventions.md` |
| Canvas / any UI | `docs/05-design-system.md`, `web/src/design-system/README.md` |
| Scope / "is X in scope?" | `docs/01-scope.md`, `docs/04-decisions.md` |
| What to build next / phases | `docs/03-phases.md`, `docs/08-dev-plan.md` |
| A project term you don't know | `docs/07-glossary.md` |

The authoritative data shapes live in two contract files — read them instead of
inferring from code: **`api/openapi.yaml`** (HTTP) and
**`docs/09-graph-contract.md`** (the canvas graph JSON + HCL resolution rules).

## Hard rules (never violate — these are the project, not preferences)

1. **Stop at the branch.** Never add PR, merge, pipeline, plan or apply logic.
2. **Never touch Azure.** No ARM/SDK calls, no Azure read or write. The resource
   inventory is built by parsing `.tf` from the ADO repo, never from Azure.
3. **No write credentials in the portal.** Azure and ADO access is through one
   Workload Identity with least privilege (ADO = Contribute + Create branch on
   the target repo only). Never store Azure/DB secrets; Postgres is Entra
   (passwordless) auth.
4. **Schema-driven, never hardcoded fields.** Every resource form is rendered
   from a module's schema (`variables.tf` → JSON Schema). Adding a module must
   not require UI code changes.
5. **Generate module CALLS by default**, in the team's style, with a
   config-driven file layout (per-folder root modules — see `samples/`). A raw
   provider `resource` block is allowed only when the developer adds it
   explicitly as a raw resource node (D-040); the generator never turns a module
   call into one. If a change makes DevOps hand-write `.tf`, it has failed its
   purpose.
6. **Cross-folder references are `data` sources in the team's style** (not
   `terraform_remote_state` unless the repo already uses it). Inspect the repo
   first.
7. **Never bypass branch policy.** The branch is for review in ADO.
8. **Honest state.** Never render an unresolved value as `0`/blank/healthy. If an
   inventory value can't be evaluated, it is `unknown`.

## Working rules

- Read affected files before editing. Keep each change focused; do not refactor,
  rename or reformat unrelated code.
- Add or update tests for behavior changes, and update the matching doc in
  `docs/` (and `docs/04-decisions.md` when a decision changes — add a new D-xxx
  entry, never rewrite an old one).
- Keep secrets, tokens, real resource names, prompts and PII out of logs,
  fixtures, screenshots and commits. `samples/` is **synthetic** — keep it that
  way.
- Prefer the smallest correct change. Surface uncertainty early; if two readings
  of a request differ materially, stop and ask.

## Directory ownership (intended layout)

- `server/` — Go backend (chi + net/http). Serves the embedded SPA and the API.
  - `server/internal/hcl/` — HCL parse + generate (`hashicorp/hcl/v2`,
    `hclwrite`, `go-cty`): the graph parser, var/locals/tfvars evaluation, the
    module-call generator. **Highest-risk code; golden + round-trip tested.**
  - `server/internal/catalog/` — module registry, `variables.tf` → JSON Schema,
    layout config.
  - `server/internal/api/` — chi routes, OpenAPI handlers, request IDs, errors.
  - `server/internal/auth/` — Entra BFF (OIDC, server-side session cookie),
    group-claim → role mapping, Workload Identity token flow.
  - `server/internal/ado/` — ADO REST client (read repo, create branch via
    Pushes API). The only writer of external state.
  - `server/internal/store/` — Postgres access (`pgx` + `sqlc`), migrations
    (`goose`), canvas JSONB, drafts, review status, audit.
- `api/` — OpenAPI spec (`openapi.yaml`): the contract. Go server + TS client
  are generated from it.
- `web/` — React + TypeScript + Vite frontend (React Flow + ELK). Design system
  in `web/src/design-system/`; Azure icons in `web/public/azure-icons/`.
- `deploy/` — Helm chart for AKS; migration runs as a pre-deploy Kubernetes Job.
- `docs/` — scope, tech stack, phases, decisions, design system, conventions,
  glossary.
- `samples/` — synthetic IaC fixtures (modules + a project tree) used as parser
  fixtures, schema sources and golden-file references.
- `.claude/skills/` — repo-local skills for agents (planning, verification, PR).
- `.github/` — PR and issue templates.

## Compatibility surfaces (preserve or migrate; don't break silently)

- **Generated HCL** — style, ordering, comments and file layout. Changing the
  generator must keep PR diffs minimal; update golden files deliberately and
  explain why in the PR.
- **OpenAPI contract** (`api/openapi.yaml`) — the frontend is generated from it.
  Prefer additive changes; regenerate both sides in the same change.
- **Postgres schema & migrations** and the canvas JSONB shape — add a migration;
  never edit a released migration in place.
- **ADO interaction** and the Workload Identity's scope.
- **Entra group-claim → role mapping.**

## Verification (run before opening a PR; add commands as the code lands)

Backend (once `server/` exists):

```bash
gofmt -l . && go vet ./... && go test ./...   # includes golden + round-trip
golangci-lint run                              # if configured
```

Frontend (once `web/` exists):

```bash
npm --prefix web ci
npm --prefix web run lint
npm --prefix web run build
npm --prefix web test                          # vitest
```

Contract & data:

```bash
# regenerate server + TS client from api/openapi.yaml and fail on drift
# goose up then goose down on a throwaway DB for migration changes
```

The repo skill `.claude/skills/change-verification` runs the right subset for
the files you touched. Docs-only or metadata-only changes don't need the code
stack — say so in the PR.

The HCL generator's tests are the backbone: **golden-file** (canvas JSON → `.tf`
byte-for-byte) and **round-trip** (parse → modify → write; untouched bytes,
including comments, unchanged). Never weaken these to make a diff pass.

## Pull requests

Use `.github/PULL_REQUEST_TEMPLATE.md`. Explain the problem, the resulting
behavior, verification run, compatibility/migration notes, and residual risk.
Keep commits focused with imperative messages. Don't open a PR before the
relevant checks pass. The repo skill `.claude/skills/pr-draft` produces a
ready-to-paste block.
