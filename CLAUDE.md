# CLAUDE.md

Claude Code reads this file automatically. It is the **single guide** for working
on TerraFlow — scope, hard rules, layout, verification and PR process.

TerraFlow is a visual, drag-and-drop portal for Terraform infrastructure (`azurerm`
first, not hardcoded). People compose resources from a module catalog; a reviewer
(the project owner or co-owner) approves;
TerraFlow then generates Terraform **module calls** and opens a **branch** on the
project's GitHub repository (Azure Repos later, D-049). **The app's job ends at the
branch** — PR, pipeline, plan and apply are done by the team in the git provider.

**Read before changing anything:** `docs/01-scope.md`, `docs/02-tech-stack.md`,
`docs/04-decisions.md`, and `samples/README.md`. For UI work also read
`docs/05-design-system.md` and `web/src/design-system/README.md`. Conventions are
in `docs/06-conventions.md`; the project's vocabulary in `docs/07-glossary.md`.

> Status: Phases 1-7 are built and run locally against `samples/` and a local
> repo. Phase 8 (GitHub, user-owned projects, sign-in with GitHub and Entra,
> D-049 and D-050) is the current work (`docs/03-phases.md`). Hosting is not
> decided. `server/internal/ado/` stays empty until an Azure Repos provider;
> don't scaffold other empty shells ahead of need.

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
| What to build next / phases | `docs/03-phases.md` (status and task breakdown) |
| A project term you don't know | `docs/07-glossary.md` |

The authoritative data shapes live in two contract files — read them instead of
inferring from code: **`api/openapi.yaml`** (HTTP) and
**`docs/09-graph-contract.md`** (the canvas graph JSON + HCL resolution rules).

## Hard rules (never violate — these are the project, not preferences)

1. **Stop at the branch.** Never add PR, merge, pipeline, plan or apply logic.
2. **Never touch Azure.** No ARM/SDK calls, no Azure read or write. The resource
   inventory is built by parsing `.tf` from the project's git repo, never from Azure.
3. **Least-privilege credentials, never exposed (amended by D-049).** The portal
   holds **no Azure credential**. For git it holds only what can create a branch
   on the chosen repository: a GitHub App installation id (tokens last one hour
   and are never stored). A user's own token is used during the request that needs
   it and discarded. Credentials are never returned to the UI, logged or committed;
   the App private key, the database password and other secrets live only in the
   host's environment. No pasted personal tokens.
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
7. **Never bypass branch policy.** The branch is for review in the git provider.
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
  - `server/internal/auth/` — BFF sessions and sign-in providers (Entra OIDC today;
    GitHub OAuth in Phase 8), global admin (`TF_ADMIN_USERS`, Entra group), project
    roles (owner, co-owner, contributor).
  - `server/internal/gitprovider/` — (planned, Phase 8) the provider interface and
    its `github` and `local` implementations (Azure Repos later): list the user's
    repos, list trees, read files at a commit, create a branch with one commit.
    The only writer of external state.
  - `server/internal/ado/` — empty until an Azure Repos provider exists.
  - `server/internal/store/` — Postgres access (`pgx`, hand-written SQL),
    migrations (`goose`), canvas JSONB, drafts, review status, audit.
  - `server/internal/localrepo/` — local git branch writer; becomes the `local`
    provider (development and tests) in Phase 8.
- `api/` — OpenAPI spec (`openapi.yaml`): the contract. Handlers and TS types are
  written by hand and must change in the same commit as the spec.
- `catalog/` — module catalog seed (JSON Schema files, D-015).
- `web/` — React + TypeScript + Vite frontend (React Flow + ELK). Design system
  in `web/src/design-system/`; Azure icons in `web/public/azure-icons/`.
- `deploy/` — Helm chart for a local cluster (kept). Hosting for go live is not decided.
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
- **Git provider interaction**, the scope of the credentials it holds, and the project role model (D-050).
- **Entra group-claim → role mapping.**

## Verification (run before opening a PR; add commands as the code lands)

Backend (set `TF_TEST_PG_DSN` to a Postgres where tests may create databases, or the store/API Postgres tests are skipped):

```bash
gofmt -l . && go vet ./... && go test ./...   # includes golden + round-trip
golangci-lint run                              # if configured
```

Frontend:

```bash
npm --prefix web ci
npm --prefix web run lint
npm --prefix web run build
npm --prefix web test                          # vitest
```

Contract & data:

```bash
# api/openapi.yaml: update the spec, handlers and web/src/types together (no codegen yet)
# migrations: covered by TestMigrationsGoDownAndUpAgain (needs TF_TEST_PG_DSN)
```

The repo skill `.claude/skills/change-verification` runs the right subset for
the files you touched; `scripts/verify.sh` runs everything. Docs-only or metadata-only changes don't need the code
stack — say so in the PR.

The HCL generator's tests are the backbone: **golden-file** (canvas JSON → `.tf`
byte-for-byte) and **round-trip** (parse → modify → write; untouched bytes,
including comments, unchanged). Never weaken these to make a diff pass.

## Local setup

- Backend: Go (stable), `gofmt`, `go vet`, ideally `golangci-lint`. Frontend: Node
  LTS, `npm --prefix web ci`.
- Postgres: set `TF_TEST_PG_DSN` (for example
  `postgres://postgres@localhost:5432/postgres?sslmode=disable`). Store and API
  tests create and drop their own databases and are **skipped** without it.
- Settings: copy `.env.example` to `.env.local` (ignored by git). The server reads
  `.env.local`, then `.env`, from the folder it starts in; a real environment
  variable always wins. Keep keys, secrets and tokens only there or in the host's
  environment, and key files outside the repository.
- No Azure or git-provider access is needed for most work: use `samples/` and the `local` provider. Never commit secrets, tokens,
  real resource names or prompts; `samples/` stays synthetic.
- Report security issues privately, see `SECURITY.md`.

## Pull requests

Use `.github/PULL_REQUEST_TEMPLATE.md`. Explain the problem, the resulting
behavior, verification run, compatibility/migration notes, and residual risk.
Keep commits focused with imperative messages. Don't open a PR before the
relevant checks pass. The repo skill `.claude/skills/pr-draft` produces a
ready-to-paste block.
