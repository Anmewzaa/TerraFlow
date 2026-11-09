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
| 7. Security improvements | **Done** (2026-11-09, local build; see the hand-over list) | Security review (threat model, secret handling, identity scope), RBAC edge cases, error/empty/loading states, perf on 80+ node canvases, accessibility, DevOps user manual, open issues | Ready for production use |
| **8. GitHub and user-owned projects (major, v3)** | **Next** (D-049, D-050) | GitHub as the git provider with a two-layer connection (the user installs the GitHub App, then connects only repositories they can access); any signed-in user creates a project from their own repository, becomes its owner and invites co-owners and contributors; sign-in with GitHub and with Entra; a flexible, provider-neutral catalog with project-level schemas and a schema editor. Hosting is not decided (go live later); Azure Repos later | M1: with the GitHub App on a personal account, user A creates a project from their own repository, invites B, B publishes, A approves, and the branch appears on GitHub; a repository the user cannot access cannot be connected |
| 9. Edit existing, advanced (after go-live) | Not planned | Attribute-level edits that keep comments inside blocks, hand-written resources that are not module calls | Not planned yet |

**Versioning:** Phase 6 ships as a major version (v2) but phase numbers continue sequentially. Phases 1-8 are the path to production. Phase 6 neither reads from nor pushes to any repo other than `samples/` and the local repo.

**Dropped from Phase 6:** the "Approver page kept as a mock UI" item (6.2.5). Review uses the real DevOps role and the real Phase 5 backend (D-039). Task-level detail follows the status sections below.

**Carried into Phase 8:** DevOps sign-off of the generated `.tf` and of the 9 conventions in `samples/README.md` before a team relies on it. The Azure service plan (AKS, Azure PostgreSQL with Entra, Key Vault, Workload Identity, Azure DevOps as a service) was dropped by D-049; `internal/ado` stays an empty package until an Azure Repos provider exists.

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

**Status: done for the local build (2026-11-09). Production readiness also needs Phase 8.**

**Stays:** the local dev sign-in (`TF_AUTH_DISABLED` + `TF_DEV_LOGIN`) is kept on
purpose so both roles can be tried locally. A start-up guard stops it from being
switched on together with an Entra configuration.

**Delivered (D-046, D-047, D-048):**
- Login: PKCE, nonce check, hashed sessions, 4-hour sessions, rate limit, start-up guard
- API: security headers, restricted CORS, cross-site write refusal, project and
  changeset scoping, drafts private to their author, validated environment names,
  JSON-safe errors, bad ids answer 404/400 instead of 500
- Generator: user text cannot change the HCL (`${` and `%{` escaped, names validated)
- UI: contrast fixed in both themes, `main` landmark and `h1` on every page, catalog
  failure no longer crashes the page, honest error and empty states
- Tests: Go security suites (`auth`, `api`, `hcl`, `store`); Playwright (`web/e2e`):
  axe on every main screen in both themes and the error and empty states; layout
  performance (100 and 200 nodes in about 0.1 s)
- Dependencies: `npm audit --omit=dev` clean (vite 6.4.6, react-syntax-highlighter 16.1.2)
- Docs: [10-security.md](10-security.md) (threat model, secret checklist, identity
  scope audit) and [11-user-manual.md](11-user-manual.md)

**Not done, handed on (each is listed with its reason in `docs/10-security.md`):**
- `govulncheck` could not run (the build environment blocks `vuln.go.dev`)
- Browser frame rate with 80+ nodes was not measured, only the layout time
- Container non-root settings are not tested on a cluster (no Docker daemon or
  Kubernetes here)
- Roles are not refreshed during a session (bounded by the 4-hour session)
- The Go server does not serve the SPA yet, so the SPA's own CSP is not set
- A CI job that runs the Postgres tests: D-013 keeps CI out of scope (needs a decision)
- Federated credential, passwordless Azure database, Key Vault and ADO branch-scoped
  permissions: dropped from Phase 8 by D-049 (Azure plan); a separate migration role
  stays in 8.0 and 8.5

---

## Phase 8 — GitHub and user-owned projects (major release, v3; D-049, D-050)

**Goal.** Anyone can sign in, see their own GitHub repositories, create a project from
one, invite other people, and get a real branch on GitHub. Terraform for Azure
(`azurerm`) stays the main target, but no code may hardcode it: provider, domain and icon
come from the schema.

**Not in this phase.** Hosting and go live (not decided), Azure Repos, GitLab, and
Terraform providers other than `azurerm` (the code is prepared, nothing more).

**Principles that keep hosting open.** No new code writes to disk, calls the `git`
binary or keeps state in process memory. The database is reached through one URL
setting. Migrations and the catalog seed run as a separate command, not at every start.

### M1 GitHub and user-owned projects (8.0 to 8.4)
- **8.0 Foundations.**
  - A `gitprovider` interface in `server/internal/gitprovider/`: list repositories (for
    the signed-in user), list a tree, read a file at a commit, create a branch with one
    commit, check the connection. Providers: `github` and `local` (the current
    `localrepo` and `IACPath` folders, kept for development and tests). An Azure Repos
    provider can implement the same interface later.
  - Project reading (graph, import, files, preview) and Approve go through the interface,
    not through folders and the `git` CLI. Reads are cached by commit SHA.
  - Migration 013, provider-neutral: `connections` (provider, external account, account
    type, provider-specific reference without secrets, created by, status); a project gets
    its connection, repository id and name, default branch and chosen path; project members
    get a role (`owner`, `co_owner`, `contributor`). Existing members become contributors;
    existing projects without an owner stay admin-managed.
  - Migration and catalog seed as a separate command (`terraflow migrate`); the rate limiter
    counts in the database.
- **8.1 GitHub connection, two layers.**
  - **Layer 1, installation.** Whoever owns a GitHub account or organisation installs the
    GitHub App on it and chooses the repositories (the App must be installable by any
    account; an organisation owner approves for the organisation). TerraFlow stores only the
    installation id and the account. For each call it signs a short JWT with the App's
    private key and exchanges it for an **installation token that lasts one hour**, limited
    to one repository and to `contents: write` plus `metadata: read`. Tokens are never stored;
    a 401 mints a new one once.
  - **Layer 2, repository connection.** The user authorizes with GitHub (a user token used
    during that request only, never stored). TerraFlow lists the repositories that the
    installation can see **and** that this user can access, and the user picks one. A user can
    only connect a repository they can access, so the App cannot be used to reach someone
    else's repository.
  - **Admin.** Sees every connection and project (metadata), can disconnect, transfer, force a
    re-check, and may set an allowlist of accounts (`TF_GITHUB_ALLOWED_OWNERS`; empty means any).
  - Security: threat table G1 to G10 and U1 to U8 in `docs/10-security.md`. Branch protection
    on the default branch is required in the target repository (a GitHub App cannot be limited
    to branch names).
  - Local setup: the owner creates the GitHub App (Contents read and write, Metadata read,
    webhook off, user authorization during installation, callback URL on `localhost`, any
    account) and keeps the private key outside the repository.
- **8.2 Projects owned by users.**
  - Any signed-in user creates a project: connection, repository (from the list), folder (from
    the tree). No free-text path; the server checks again that the folder exists and holds
    `.tf` files. The creator is the owner.
  - Roles: **owner** (everything, including approve, invite, set roles, disconnect, delete,
    transfer); **co-owner** (approve, invite, set roles below owner, manage the project's
    schemas; cannot delete or transfer); **contributor** (build, save, publish; cannot approve).
    The owner may approve their own changes; a per-project setting can require a second reviewer.
  - The owner invites users who have signed in; inviting someone lets them open branches on that
    repository through TerraFlow, and the screen says so.
  - **Admin visibility.** The global admin sees every project's metadata (name, owner, repository,
    members, status, audit). Opening a project's canvas or diff needs "support access", which is
    audited each time (default; D-050).
- **8.3 Branch on GitHub.** One atomic commit through the Git Data API on a new branch
  `terraflow/<project>/<env>-<id8>`; never forced, refused if the branch exists; refused with
  409 if the base commit changed since the plan. The audit entry links the commit.
- **8.4 CI.** A GitHub Actions workflow runs `gofmt`, `go vet`, `go test` with a Postgres
  service, and the web lint, typecheck, build and tests (D-013 is withdrawn by D-049).
- **Tests.** The GitHub client against `httptest` (JWT shape, token exchange, one retry on 401,
  rate-limit answers, tree and file reads, the commit sequence, branch exists, base moved);
  access tests (a user cannot connect a repository they cannot access, roles, admin
  visibility, invitation limits); a live smoke test against the owner's sandbox repository that
  runs only when the App settings are in the environment.
- **Local testing note.** Before sign-in with GitHub exists (M2), development uses the
  made-up users; the GitHub account that authorizes is recorded on the connection.
- **Exit (M1).** With the GitHub App installed on a personal account: user A creates a project
  from their own repository with the picker, invites B as contributor, B builds and publishes, A
  approves, and the branch appears on GitHub. A repository the user cannot access cannot be
  connected. The admin sees the project in the list.

### M2 Sign-in with GitHub and Entra (8.5)
- An identity-provider interface in `server/internal/auth/` with two providers.
  - `github`: OAuth2 through the same GitHub App (state and PKCE; no ID token and no nonce);
    the identity comes from `GET /user`; the token is discarded.
  - `entra`: the existing OIDC flow with an app registration in a free tenant.
- Users are identified as `github:<id>` and `entra:<oid>`; existing ids without a prefix are
  treated as Entra. **No automatic linking** of the two (it would allow account takeover).
- The login page shows a button for each enabled provider (`TF_LOGIN_PROVIDERS`).
- Global admin: `TF_ADMIN_USERS` (ids or emails); Entra group mapping stays optional.
- When an owner signs in with GitHub, TerraFlow re-checks (with the transient token) that they can
  still access their connected repositories; if not, the connection is marked "needs attention"
  and publishing is blocked until it is re-verified.
- **Exit.** Sign in with either provider and use the M1 flow as that user.

### M3 Flexible catalog and schema editor (8.6)
- **Scopes.** A global catalog (admin) and project catalogs (owner and co-owner add and edit;
  visible only to that project; contributors use them). Raw resource templates follow the same
  scopes.
- **Provider-neutral.** A schema declares its Terraform provider (`azurerm` today), its domain
  and its icon; no code branches on module names; the raw resource type check already accepts
  any `provider_name`. Azure-only assumptions in code and docs are removed or made data.
- **More `azurerm` resources.** The list is agreed before work starts.
- **Schema editor page.** Start from a ready-made example; live validation that lists every
  problem at once (builds on `catalog.Validate`); a preview of the form; a diff against the
  previous version with breaking changes marked; optionally a draft schema from a pasted
  `variables.tf`; export and import of a schema between projects.
- **Exit.** An owner adds a resource or module to their project through the page with no code
  change; a schema for another Terraform provider validates and renders (test fixture), even
  though no icon set exists for it.

### Later, not scheduled
- **Azure Repos provider.** The same interface. Differences are in the connector: no
  installation, Entra delegated tokens (or a service principal), organisation, project and
  repository levels, one Pushes API call for branch and commit.
- **Go live.** Hosting is not decided. When it is: a host, TLS and a domain (the GitHub App and
  Entra redirect URIs), the database URL, the list of environment variables, the migrate command.
- Terraform providers other than `azurerm` in generator rules, domains and icons.

### Risks and unknowns
- **One App private key can mint tokens for every installation**, now across all users' repositories.
  It lives only in the environment, is never logged, and has a documented rotation; tokens are
  limited to one repository. This is the largest single risk of the phase.
- **Anyone may sign in.** Limits per user (projects, requests) and a sign-up switch need a
  decision before go live.
- A contributor acts through the owner's connection; if the owner later loses access to the
  repository, TerraFlow only notices when the owner signs in with GitHub or re-verifies.
- GitHub API rate limits per installation; reads are cached by commit SHA.
- Admin visibility of users' projects is a privacy decision (D-050 sets the default).

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
