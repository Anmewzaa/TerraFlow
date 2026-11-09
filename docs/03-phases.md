# 03 — Phases and development plan

Status: agreed (v1, re-planned 2026-11-08; see D-034. Updated 2026-11-09: Phase 6 done)

Principles (every phase is demoable, read-only before write, and the riskiest piece, HCL parse/evaluate/generate, is proven first):

- Every phase ends in something that can be demonstrated.
- Read-only work comes before any write work.

| Phase | Status | Work | Exit criteria |
|---|---|---|---|
| 1. Visualizer (local) | Done | Go parser: `.tf` folders → graph JSON; React Flow + ELK | `samples/iac/AIC` renders as the expected graph |
| 2. Foundation (local) | Done | Entra BFF login, Projects, Postgres, chi, Helm | Login → projects → graph, read-only |
| 3. Module catalog | Done | `variables.tf` → JSON Schema → form; layout config | New module needs no UI code change |
| 4. Canvas editing + drafts | Done (DevOps sign-off of generated `.tf` pending) | Drag and drop, locked derived values, HCL generator, Save/Publish | Generated `.tf` matches what the team writes by hand |
| 4.5 UI overhaul | Done | Harbor design system across all views, light/dark | Semantic tokens only; matches mockup |
| 5. Review + branch (local) | **Done** (2026-11-08) | Review screen, Approve → branch on local repo, Reject with reason, audit log, separation of duties | Full flow works end to end against a local test repo |
| 6. Features + UI v2 (major) | **Done** (2026-11-09, local) | App functions and UI rework against `samples/` and the local repo. 6.0 safe generator (merge, never replace) and the AIC/AIH mock projects; 6.1 Home with project CRUD in Postgres and editing of resources that already exist in the repo (D-036, D-038); 6.2 canvas v2 (catalog and resources panels, nested boards, arrows between resources, full-page HCL preview, raw resources an admin prepares, D-039..D-041); 6.3 Admin API and UI (modules and strict schema import, projects, members, users, raw resources, global audit, D-037, D-042); plus a one-pending-changeset publish lock, local sign-in, and Save-as-version (D-043, D-044) | Project/module/resource/user data lives only in Postgres; the canvas generates correct HCL without ever overwriting existing files; schema import validates strictly; golden + round-trip tests cover the new cases |
| **7. Security improvements** | **In progress** (D-046, D-047) | Security review (threat model, secret handling, identity scope), RBAC edge cases, error/empty/loading states, perf on 80+ node canvases, accessibility, DevOps user manual, open issues | Ready for production use |
| **8. ADO + real resources** | Planned | `internal/ado` (read repo `.tf` from `main`, create branch via Pushes API) behind the same interfaces as `localrepo`; Workload Identity, Azure PostgreSQL (Entra passwordless), Key Vault, federated credential, deploy to AKS | Publish, approve, then a branch appears in a real ADO repo; app runs on AKS with no local secrets |
| 9. Edit existing, advanced (after go-live) | Not planned | Attribute-level edits that keep comments inside blocks, hand-written resources that are not module calls | Not planned yet |

**Versioning:** Phase 6 ships as a major version (v2) but phase numbers continue sequentially. Phases 1-8 are the path to production. Phase 6 neither reads from nor pushes to any repo other than `samples/` and the local repo.

**Dropped from Phase 6:** the "Approver page kept as a mock UI" item (6.2.5). Review uses the real DevOps role and the real Phase 5 backend (D-039). Task-level detail follows the status sections below.

**Carried into Phase 7/8:** DevOps sign-off of the generated `.tf` (Phase 4 exit); the Entra app still uses a client secret locally, and the federated credential arrives with Phase 8; `internal/ado` is an empty package until Phase 8.

## Parallel work during Phase 1

Request access early, because approvals in a bank take time:

- Entra app registration, federated credential and group assignment
- Workload Identity (user-assigned managed identity) and its permissions in the ADO org
- Azure PostgreSQL Flexible Server (Entra auth) and Key Vault
- A test ADO repo for Phase 8 (request now; needed for Phase 8)

## Before Phase 1

- DevOps reviews the conventions table in `samples/README.md` and corrects the samples where they differ from the real repos.

---

# Task breakdown per phase

Legend: **BE** backend (Go) · **FE** frontend (React) · **INF** infra/data ·
**T** tests · **Exit** done-when.

---

## Phase 0 — Skeleton (enabling, ~small)
Stand up the repo so later phases have somewhere to land. No product behavior.

- **BE** `go mod init`, `server/` layout, a `server/cmd/visualize` CLI entry, `slog` setup.
- **FE** Vite + React + TS app in `web/`, design system wired (`tokens.css`), ELK + React Flow installed, Azure icons loaded from `web/public/azure-icons/`.
- **INF** `api/openapi.yaml` stub; local verification script (gofmt/vet/test, web lint/build); `.env.example`.
- **Exit** `go test ./...` and `npm --prefix web run build` both run green on an empty skeleton.

> Start the **access requests in parallel here**: Entra app registration +
> federated credential + group assignment, a user-assigned managed identity with
> ADO Contribute+Create-branch on a test repo, Azure PostgreSQL Flexible Server
> (Entra auth), Key Vault. These have long lead time at a bank.

---

## Phase 1 — Visualizer (local, read-only) — the core
Prove the HCL engine on `samples/`. No auth, DB or ADO yet.

- **BE `internal/hcl` (highest risk):**
  - Parse a folder tree of `.tf` (`hashicorp/hcl/v2` / hclsyntax) → AST.
  - Extract module calls (source + `?ref=` version + inputs), resources, `data`
    sources, `locals`, `variables`, `outputs`, providers incl. aliases.
  - Evaluate `var` / `locals` / `.tfvars` with `go-cty` to resolve names where
    possible; anything unresolved is marked **unknown** (never defaulted).
  - Build the graph: nodes per folder (module calls, data sources), edges for
    same-folder references (module outputs) and **cross-folder references**
    (match a `data` source to its producer by the reconstructed name from
    `naming.tf` locals).
  - Classify each node's **resource domain** (network/compute/data/security/
    neutral) for color + icon.
  - Serialize to a stable **graph JSON** — this is the contract to the FE.
  - Mark hub/provider-alias lookups as **external**.
- **FE:** render graph JSON with React Flow + ELK nested layout (RG ⊃ VNet ⊃
  subnet), Azure icon + domain accent per node, edges for `data` links, external
  styling; node detail panel (inputs, resolved values, `unknown`). Pan/zoom,
  select. Read-only.
- **T:** golden tests — `samples/iac/AIC` (and AIH) → expected graph JSON (incl.
  cross-folder + hub-as-external, matching `samples/README.md`). Unit tests for
  the var/locals/tfvars evaluator (the risky bit) with small fixtures.
- **Exit:** open the sample project per env (dev/prod) and get the exact graph
  in `samples/README.md`, including cross-folder references and hub externals.

---

## Phase 2 — Foundation (auth · projects · repo read · deploy)
Everything real but still read-only.

- **BE / contract:** define `api/openapi.yaml` for `GET /me`, `GET /projects`,
  `GET /projects/{id}/graph`; generate Go server + TS client from it.
- **BE auth (Entra BFF):** OIDC auth-code + PKCE, server-side session in an
  encrypted httpOnly cookie, **group claim → role** (DevOps/Developer) and
  project membership; detect the group-**overage** claim and fail with a clear
  message (no Graph call).
- **BE projects:** Project = one ADO repo + one Entra group; list only the
  projects the user's groups allow.
- **BE ADO client (`internal/ado`):** read a repo's `.tf` from `main` → feed the
  Phase 1 parser → graph.
- **BE store (`internal/store`):** `pgx` + Workload Identity passwordless
  (`BeforeConnect` token refresh), `goose` baseline migration, `sqlc` setup.
- **INF:** single container image (Go `embed`s the built SPA), distroless base,
  Helm chart on AKS, migration **Job** pre-deploy, Key Vault CSI for the cookie
  key, Workload Identity bindings.
- **T:** integration tests with a real Postgres (container) and a mocked ADO
  (`httptest`); auth tests for role mapping + overage.
- **Exit:** a developer logs in with Entra, sees only their projects, and views
  the graph of a **real** repo. Still read-only. Running on AKS.

---

## Phase 3 — Module catalog & schema-driven forms
No UI code per resource type.

- **Decision at phase start (made: D-015, D-019):** modules are admin-registered
  JSON Schema files with the UI metadata inside; the schema is not extracted from
  `variables.tf` at runtime.
- **BE `internal/catalog`:** module registry (module, version, source, schema,
  UI metadata, layout mapping, allowed-per-project); `variables.tf` → **JSON
  Schema** (types, defaults, `validation`, descriptions) + UI overlay (labels,
  dropdowns, hidden/derived).
- **BE API:** list catalog for a project, get a module's schema.
- **FE:** catalog panel; render forms from JSON Schema with
  a hand-written `SchemaForm` with custom widgets (pick-existing-resource
  dropdown fed by the inventory, locked/derived fields). `react-jsonschema-form`
  is not used.
- **T:** schema-conversion unit tests on the sample modules; a form renders for
  a module with **no FE code change**.
- **Exit:** add a new module through the chosen mechanism and its form appears
  with no UI code change.

---

## Phase 4 — Canvas editing + draft + generator
**UI design conversation happens before this phase.** First write path.

- **BE generator (`internal/hcl`, second risky piece):** canvas model → **module
  call** blocks + the `data` blocks for cross-folder refs, in the team's style
  and the **config-driven layout** (per-folder root modules). Built with
  `hclwrite` so comments/order/format survive.
- **BE drafts:** canvas as JSONB, status Draft/Pending; `Save` / `Publish`
  endpoints + migration.
- **FE:** drag/drop modules onto the canvas, connect to existing resources
  (derived RG **locked**), edit inputs via the schema forms, Save draft /
  Publish; preview of the generated HCL (a diff view built with
  `react-syntax-highlighter`; Monaco is not used).
- **T:** **golden-file** (canvas JSON → `.tf`, byte-for-byte) and **round-trip**
  (parse existing → add one module → write; untouched bytes incl. comments
  unchanged). These are the backbone — never weakened to pass a diff.
- **Exit:** DevOps signs off that generated `.tf` matches what they would write
  by hand on the sample repo.

---

## Phase 5 — Review + branch + audit (local) — DONE 2026-11-08
Review screen (canvas + HCL diff), Approve → branch in a local git repo
(`internal/localrepo`), Reject with required reason, audit log, optional
separation of duties (D-026, D-027). Real ADO is Phase 8 (D-034).

---

## Phase 6 — Features + UI v2 (major, local only) — DONE 2026-11-09
No ADO read/push. Everything runs against `samples/` and the local repo. The
reasoning for each item is in the decision log; this list only records what was
built and where.

- **6.0 Safe generator + samples (D-035, D-036).** `hcl.Apply` merges into existing
  files and never rewrites one wholesale; a duplicate module label fails instead of
  overwriting (regression for the `rg.tf` incident). `samples/` reworked into the
  mock projects AIC and AIH; graph output made deterministic.
- **6.1 Existing resources are editable (D-036, D-038).** `GET /projects/{id}/import`
  returns repository module calls as nodes with `existing`; an edit replaces only
  that block. **Home (D-042, D-043):** project create / rename / delete in Postgres,
  project cards, search; delete never touches repository files.
- **6.2 Canvas v2 (D-039..D-041, D-044).** Left panel with Modules and Resources
  tabs; nested boards; reference arrows (same-folder reference or `data` block,
  decided by the backend, D-033); raw resources are admin-prepared templates
  (D-040, D-041); a full-page HCL preview that shows only what changed (D-042);
  Save makes a version and Publish happens from Changesets (D-044).
- **6.3 Admin (D-037, D-042).** API: users, project members, project CRUD, component
  settings, module import with strict validation and publish/deprecate, raw resource
  templates, global audit log. UI: Admin and Audit log tabs on Home, admin only.
- **Also in this phase:** one changeset waiting for review per project (D-043);
  local sign-in by picking a user (D-043); Helm chart with its own Postgres and a
  workspace volume.
- **Dropped:** "Approver page as a mock" (D-039). Review uses the real devops role.
- **Contract:** endpoints in `api/openapi.yaml` (0.6.0); migrations 008-012 only add.
- **Exit:** see the status table above (Phase 6).

---

## Phase 7 — Security improvements
Threat model, secret-handling checklist, identity-scope audit, RBAC edge cases
(incl. new in-app project assignment), error/empty/loading states, perf on 80+
nodes, accessibility, DevOps user manual. **Exit:** ready for production use.

**Stays:** the local dev sign-in (`TF_AUTH_DISABLED` + `TF_DEV_LOGIN`) is kept on
purpose so both roles can be tried locally. Phase 7 only adds a guard so it cannot
be switched on together with a real Entra configuration.

**Test coverage added (D-046):** `server/internal/auth/security_test.go` (role
mapping, login redirect and cookie flags, callback state check, session
middleware, logout, dev sign-in), `server/internal/api/security_matrix_test.go`
(walks every route: sign-in required, admin routes refuse developers, project
routes hide projects from non-members, changesets scoped to their project, body
size and malformed input) and `server/internal/hcl/injection_test.go` (user values
cannot change the generated HCL).

**Done in Phase 7 so far (D-047):** PKCE (S256) and the ID token nonce check on
login; session ids stored as SHA-256 and expired sessions purged; CORS limited to
`TF_ALLOWED_ORIGINS` (none by default); security headers on every response; a
second CSRF layer that refuses cross-site writes; a rate limit on sign-in and
callback; the server refuses to start with `TF_AUTH_DISABLED=true` and Entra
settings together.

**Still open:**
- Refresh roles during a session (a user removed from an Entra group keeps the
  role until the session ends, up to 8 hours)
- Drafts: a project member can open another member's draft by id (D-044 says drafts
  belong to their author)
- The SPA is not served by the Go server yet, so its own CSP is not set
- Rate limit is per pod and trusts `X-Forwarded-For` (set by the ingress)
- Drop the default DB password and the client secret (Phase 8)
- Threat model, identity-scope audit, DevOps user manual
- Error/empty/loading states, performance on 80+ nodes, accessibility
- Browser (E2E) tests, a CI job that runs the Postgres tests (D-013 keeps a CI
  pipeline out of scope, so this needs a decision), dependency and image scanning,
  `go test -fuzz` on the generator inputs

---

## Phase 8 — ADO + real resources
`internal/ado` (read `.tf` from `main`, create branch via Pushes API, Contribute +
Create-branch only) behind the same interfaces as `localrepo`; Workload Identity,
Azure PostgreSQL (Entra passwordless), Key Vault, federated credential, Helm on AKS.
**Exit:** publish, approve, branch in a real ADO repo; no local secrets.

---

## Phase 9 — Edit existing resources (post-GA)
Modify existing/hand-written resources. Highest merge risk, intentionally after
go-live.

---

## Sequencing & risk
- **Critical path:** P0 → P1 (HCL engine) → P2 → P3 → P4 (generator) → P5 → P6 → P7 → P8.
- **Parallelizable:** access/provisioning requests during P0–P1; Helm/deploy
  (P2 INF) alongside P2 BE; the UI-design conversation during P3 so P4 can start
  clean.
- **Biggest risks, proven early:** (1) evaluating `var`/`locals`/`.tfvars` to
  resolve names in P1; (2) generating HCL that matches the team's style
  byte-for-byte in P4. Both are guarded by golden + round-trip tests.
- **Needs a real sample before coding:** confirm the 9 conventions in
  `samples/README.md` against the real repo (DevOps) before P1.
