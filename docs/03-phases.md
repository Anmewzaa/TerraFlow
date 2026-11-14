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
| **8. GitHub and user-owned projects (major, v3)** | **In progress** (D-049 to D-053; M1 and M2 built, not yet run against real GitHub; M3 not started) | GitHub as the git provider with a two-layer connection (the user installs the GitHub App, then connects only repositories they can access); any signed-in user creates a project from their own repository, becomes its owner and invites co-owners and contributors; sign-in with GitHub and with Entra, with admin approval of new users; a flexible, provider-neutral catalog with project-level schemas and a schema editor. Hosting is not decided (go live later); Azure Repos later | M1: with the GitHub App on a personal account, user A creates a project from their own repository, invites B, B publishes, A approves, and the branch appears on GitHub; a repository the user cannot access cannot be connected |
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

**Stays:** the local dev sign-in (`TF_AUTH_DISABLED=true` + `TF_DEV_LOGIN=true`) is kept on
purpose so both roles can be tried locally. A start-up guard stops it from being
switched on together with an Entra configuration or sign-in policy settings (D-063).

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

### Status 2026-11-10 (run against real GitHub, then M1b, M1c and M2 built)

| Item | State |
|---|---|
| Connect GitHub with the real App, 97 repositories listed, folders read | **Done and seen working** (reading a repository: 1 to 2 s; was 37 s, see D-053) |
| M1b Repository list instead of a drop-down (search, private marks, branch) | **Done** (`RepoPicker`); not yet clicked through by the owner |
| M1b Home > GitHub settings (my connections, refresh, choose repositories, disconnect) | **Done** (`GET /github/connections`, `POST /github/connections/{id}/disconnect`) |
| M1c New folder for a repository with no Terraform yet | **Done** (D-054); an empty repository (no commits) is refused with a clear message |
| D-053 one way to create projects per server (`TF_PROJECT_SOURCE`) | **Done** |
| M2 Sign in with GitHub (state + PKCE, numeric id) and Entra through one policy | **Done and tested with the fake GitHub; not yet run against real GitHub** |
| M2 Sign-up approval, admin sign-ups tab, suspension ends sessions at once | **Done** (D-055, migration 016) |
| M2 Re-check a project owner's repository access at GitHub sign-in | **Done** (D-061, migration 020; also at Connect GitHub); tested with the fake GitHub |
| M2 `TF_AUTO_APPROVE` rules (Entra tenant, GitHub organisations, e-mail domains) | **Done** (D-062); tested with the fake GitHub, not yet against real GitHub |
| Dev-only switches (`TF_DEV_LOGIN` off by default, refused beside real sign-in settings, local projects for development) | **Done** (D-063) |
| D-056 Initialize an empty repository (after confirmation); New project goes straight to GitHub when not connected; dialog redesigned in steps | **Done** |
| M3a Admin redesign: sections with Docs drawer, module import in steps with an example, raw resource presets, project overview, People and access | **Done** (`docs/12-schema-reference.md`, `GET /admin/projects/overview`) |
| M3b Project settings: variables (`{org}` becomes `var.org`), `variables.tf`, `env/<env>.tfvars`, starter files, default layout, Settings screen, Advanced step in New project | **Done** (D-057, migration 017); tested through preview against the running app; golden files unchanged |
| M4a Cloud providers: registry with logo and versions, provider and version per project, providers.tf, raw resources with provider, category and version range and filtered to the project, module provider and category | **Done** (D-058, migration 018) |
| M4b Palette (search, categories, provider header, module version choice), one Project page for Team and Settings, Docs as a page | **Done** (D-059) |
| M3c Catalog scopes and schema editor (on top of the provider model) | **Done** (D-060, migration 019); tested with Postgres and through the running app with a local project; the list of more `azurerm` modules is still to be agreed |
| Create, publish, approve, branch on GitHub with a real project (exit test of M1) | **Not run yet** |

### Progress 2026-11-13 (M3c built; what is left of Phase 8)

Built and tested locally: M1, M1b, M1c, M2 (against the fake GitHub), M3a, M3b, M3c (D-060), M4a, M4b.
Checks on 2026-11-13: `go test ./...` with Postgres (migration 019 up and down, golden files unchanged),
vitest (109 tests), the browser tests (23 tests), and the project catalog clicked through in the running
app with a local project.

Left, in the order to do it:

1. **Needs real GitHub (the owner's App and cluster):** the exit test of M1 (create, publish, approve,
   branch), sign-in with GitHub (M2), and `TestLiveGitHub`. BUG-1 did not come back (below).
2. **M2:** built on 2026-11-13 (D-061, D-062); what is left is the run against real GitHub, which also needs
   the App permissions the `TF_AUTO_APPROVE` rules ask for (Members: read, Email addresses: read).
3. **Dev-only switches:** built on 2026-11-13 (D-063).
4. **M3, to agree first:** the list of further `azurerm` modules for the global catalog.
5. **Known, outside this phase's code:** `api/openapi.yaml` does not parse as YAML at line 294 (an
   unquoted description, older than M3c); the palette badge does not yet tell a project entry that
   hides a global one apart from any other project entry (only the tooltip does).

### M1 GitHub and user-owned projects (8.0 to 8.4)

**Status (2026-11-09): built, not yet run against real GitHub.** Everything below is implemented and
tested against a fake GitHub that enforces the real protocol; the live smoke test with the owner's
sandbox repository and the GitHub App has not been run (it needs the App's credentials). Left open:
the live smoke test (needs the App credentials; the test exists and is skipped without them).

| Item | State |
|---|---|
| 8.0 `gitprovider` interface, `local` and `github` sources, an in-memory source for tests | **Done**, with a contract suite that every source passes |
| 8.0 Graph, import, files, preview and Approve read and write through the provider | **Done**; behavior for local projects is unchanged (all earlier tests pass) |
| 8.0 `hcl` reads from `fs.FS` (a folder or a snapshot), so no disk is needed | **Done**; the in-memory graph equals the folder graph for both sample projects |
| 8.0 Migration 013 and the store: connections, project repository link, roles | **Done**; used by the API since 8.1 and 8.2 |
| 8.0 Reads cached by commit SHA | **Done**: snapshots by tree SHA and blobs by blob SHA, bounded, in memory (immutable data, so never stale; `github/cache.go`) |
| 8.0 Migration and catalog seed as a separate command | **Done**: `terraflow migrate`, and `TF_MIGRATE_ON_START=false` skips them at start |
| 8.0 Rate limiter in the database | **Done**: migration 015, `Queries.TakeToken`; falls back to the in-memory limiter if the database fails |
| 8.1 GitHub App client: signed JWT, one-hour tokens limited to one repository, snapshot, branch with one commit | **Done**, tested against a fake that enforces the real protocol |
| 8.1 Connect flow (authorize, return, state check, the user's installations from their own token) | **Done**, with a fake GitHub (`/api/github/authorize`, `/callback`) |
| 8.1 Layer 2: list the repositories the user can access and connect one | **Done**: short-lived grants (migration 014), repository and folder pickers, `POST /projects/from-repo` |
| 8.1 Admin connections page, allowlist `TF_GITHUB_ALLOWED_OWNERS` | **Done**: list, check with GitHub, disconnect; allowlist enforced in the callback |
| 8.2 Roles enforced in the API (approve, invite, set roles), owner creates projects | **Done**: owner and co-owner approve; invite, roles, remove, transfer, delete; the admin's replace-all member list never removes the owner |
| 8.2 Project wizard, members screen, admin support access, second reviewer | **Done**: GitHub dialog, Team screen, `support.access` audited on every admin request to a project they are not a member of, per-project second-reviewer switch |
| 8.3 Branch on GitHub from Approve | **Done**: a GitHub project reads and branches through the App (`api.ProjectRepos`); a broken connection stops the plan instead of reading nothing |
| 8.4 GitHub Actions CI | **Done** (`.github/workflows/ci.yml`, with a Postgres service); not yet seen running on GitHub |
| Catalog `source` lines for `infrastructure-terraform-modules` | **Done**: schemas carry a full `x-tf-module.source`; a bare name goes under `TF_MODULE_SOURCE_BASE`. The `//modules/<name>` layout is an assumption to confirm against the real repository |
| Browser tests for the new screens | **Done**: `web/e2e/github.spec.ts` (dialog states, create request, return-from-GitHub error, admin connections), GitHub is mocked |
| Live smoke test | Written (`TestLiveGitHub`, skipped unless `TF_LIVE_GITHUB_REPO`, `TF_LIVE_GITHUB_INSTALLATION_ID` and the App key are set); **not run**: no App credentials where it was written |

- **8.0 Foundations.**
  - `server/internal/gitprovider/`: the `Source` interface (read a snapshot of the project
    folder at a commit; create one commit on a new branch). Implementations: `Local` (a folder
    and the `git` binary, for development and tests), `github`, and `Memory` (no disk, no
    binary, no network). Listing a user's repositories belongs to the connect flow (8.1), not to
    `Source`. An Azure Repos provider can implement the same interface later.
  - `gitprovidertest.Run` is the contract every source must pass (reading, adding and updating
    files, the error cases, line endings, unsafe paths); `RunBase` adds what remote sources
    guarantee (the branch is made from the planned commit even if the default branch moved).
  - The API never opens a project's folder itself: handlers get a `Repos` function that returns
    the project's `Source`. A repository that cannot be read stops the plan; it is never treated
    as empty.
  - Migration 013, provider-neutral: `connections` (provider, external id, account, status,
    creator; no secret), `project_repos` (connection, repository id and name, default branch,
    folder, who connected it, second-reviewer switch), and a role on `project_members`
    (`owner`, `co_owner`, `contributor`; at most one owner per project, enforced by an index).
    Existing members become contributors; existing projects keep working as local projects.
  - Still to do here: reads cached by commit SHA; migration and catalog seed as a separate
    command (`terraflow migrate`); the rate limiter counting in the database.
- **8.1 GitHub connection, two layers.**
  - **Layer 1, installation.** Whoever owns a GitHub account or organisation installs the
    GitHub App on it and chooses the repositories (the App must be installable by any
    account; an organisation owner approves for the organisation). TerraFlow stores only the
    installation id and the account. For each operation it signs a JWT (nine minutes, a
    minute in the past to tolerate clock drift) with the App's private key and exchanges it for an
    **installation token that lasts one hour**, limited to one repository and to `contents: write`
    plus `metadata: read`. Tokens are never stored or reused; a 401 mints a new one once.
    *(Built.)*
  - **Layer 2, repository connection.** The user authorizes with GitHub (a user token used
    during that request only, never stored). TerraFlow lists the repositories that the
    installation can see **and** that this user can access, and the user picks one. A user can
    only connect a repository they can access, so the App cannot be used to reach someone
    else's repository. *(Not built.)*
  - **Admin.** Sees every connection and project (metadata), can disconnect, transfer, force a
    re-check, and may set an allowlist of accounts (`TF_GITHUB_ALLOWED_OWNERS`; empty means any).
    *(Not built.)*
  - Security: threat table G1 to G10 and U1 to U9 in `docs/10-security.md`, which says which are
    implemented. Branch protection on the default branch is required in the target repository (a
    GitHub App cannot be limited to branch names).
  - Local setup: the owner creates the GitHub App (Contents read and write, Metadata read,
    webhook off, user authorization during installation, callback URL on `localhost`, any
    account) and keeps the private key outside the repository.
- **8.2 Projects owned by users.** *(Not built, except the role tables.)*
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
  - Approve today still needs the global `devops` role; it moves to owner and co-owner when this
    item lands.
- **8.3 Branch on GitHub.** One atomic commit through the Git Data API (blobs, tree, commit,
  reference) on a new branch `terraflow/<project>/<env>-<id8>`. The branch is made from the commit
  the plan was built on, so it holds exactly what the reviewer saw; if the default branch has
  moved since, GitHub shows the difference in the pull request. It never moves an existing
  reference and is refused if the name is taken. *(Built in the provider; the audit entry will link
  the commit when projects use it.)*
- **8.4 CI.** A GitHub Actions workflow runs `gofmt`, `go vet`, `go test` with a Postgres
  service, and the web lint, typecheck, build and tests (D-013 is withdrawn by D-049).
- **Tests.**
  - Done: the contract suite on all three sources; the GitHub client against a fake that checks
    the App JWT (signature, issuer, lifetime), that a token is known, unexpired and limited to its
    repository and permissions, plus 401 retry, rate limit, missing things, unreadable and
    oversized repositories, bad input refused before any request, and that no secret appears in an
    error or a printed value; API tests on an in-memory repository; store tests on Postgres.
  - To do: access tests (a user cannot connect a repository they cannot access, roles, admin
    visibility, invitation limits); Playwright tests for the new screens; a live smoke test against
    the owner's sandbox repository that runs only when the App settings are in the environment.
- **Local testing note.** Before sign-in with GitHub exists (M2), development uses the
  made-up users; the GitHub account that authorizes is recorded on the connection.
- **Exit (M1).** With the GitHub App installed on a personal account: user A creates a project
  from their own repository with the picker, invites B as contributor, B builds and publishes, A
  approves, and the branch appears on GitHub. A repository the user cannot access cannot be
  connected. The admin sees the project in the list.

### M2 Sign-in with GitHub and Entra (8.5)

**Status (2026-11-09): built, not yet run against real GitHub or Entra.** Tested against the fake GitHub and a real
Postgres. Done: sign-in with GitHub (`/auth/github/login` and `/callback`, state check, numeric id as identity, the token
dropped at once); `TF_LOGIN_PROVIDERS` and `GET /api/auth/providers` (the login page draws a button for each; one
provider with nothing to report redirects straight to it); migration 016 (`provider`, `status`, reason, who, when);
`TF_SIGNUP_MODE` (`approval` default, `open`, `invite_only`), `TF_ADMIN_USERS` (ids or emails, active at once with the admin
role), `TF_SIGNUP_MAX_PENDING` (default 200); the status is checked on every request (a pending user may only call
`/api/me`; suspending or rejecting deletes the user's sessions); admin approve, reject, suspend and restore with a reason,
audited (`POST /admin/users/{oid}/status`); a pending, rejected or suspended user cannot be invited or assigned; the
waiting page and the approval list in the admin Users tab; tests in `auth`, `api`, `store` and `web/e2e/signup.spec.ts`.
Not done: `TF_AUTO_APPROVE` rules (Entra tenant, GitHub organisations, verified email domains); the re-check of an
owner's repository access when they sign in with GitHub (connection "needs attention"); limits per user for the `open`
mode; a rejected id is blocked only until an admin restores it (no separate block list); a suspended owner's project is
not yet stopped from publishing and approving. GitHub App settings: add `TF_GITHUB_LOGIN_REDIRECT_URI`
(default `http://localhost:8080/auth/github/callback`) to the App's callback URLs.
- An identity-provider interface in `server/internal/auth/` with two providers.
  - `github`: OAuth2 through the same GitHub App (a random `state` checked on return; no PKCE, ID token or nonce);
    the identity comes from `GET /user`; the token is discarded.
  - `entra`: the existing OIDC flow with an app registration in a free tenant.
- Users are identified as `github:<id>` and `entra:<oid>`; existing ids without a prefix are
  treated as Entra. **No automatic linking** of the two (it would allow account takeover).
- The login page shows a button for each enabled provider (`TF_LOGIN_PROVIDERS`).
- Global admin: `TF_ADMIN_USERS` (ids or emails); Entra group mapping stays optional.
- **Sign-up approval (D-050).** `TF_SIGNUP_MODE=approval` is the default: a new user is
  `pending` after the first sign-in and sees only a "waiting for approval" page. A pending user
  cannot create a project, connect a repository, be invited into a project, or receive any
  token. The admin has a list of pending users with approve, reject (with a reason) and
  suspend (which ends their sessions at once); all audited. Other modes: `open` (with
  per-user limits) and `invite_only`. `TF_ADMIN_USERS` are active at once; optional
  auto-approval rules (`TF_AUTO_APPROVE`: the Entra tenant, GitHub organisations, verified
  email domains). People are identified by their numeric GitHub id or Entra object id, never by a
  login name. The number of pending users is capped. A rejected id cannot sign up again until
  an admin lifts it; a suspended owner's project stops publishing and approving until an admin
  transfers it.
- When an owner signs in with GitHub, TerraFlow re-checks (with the transient token) that they can
  still access their connected repositories; if not, the connection is marked "needs attention"
  and publishing is blocked until it is re-verified.
- **Exit.** Sign in with either provider and use the M1 flow as that user.

### Phase 8 bug list and next run (first run against real GitHub, 2026-11-09)

**State left in the local cluster** (`docker-desktop`, namespace `terraflow-local`): image
`terraflow:v0.3.0-local` (the code of M1), Secret `tf-github` (keys `private-key.pem` and `client-secret`; the key
file is `~/.terraflow/github-app.pem`, outside the repository), settings `TF_GITHUB_APP_ID=5682033` (set with
`--set-string`), `TF_GITHUB_APP_CLIENT_ID`, `TF_GITHUB_APP_SLUG=mewstellaflow`,
`TF_GITHUB_REDIRECT_URI=http://localhost:8080/api/github/callback`, `TF_WEB_URL=http://localhost:5173`. The server
logs `GitHub App configured`, so the key parses. The target is `https://github.com/Anmewzaa/demo-terraform`
through the App `mewstellaflow`. The screens run on the Vite dev server (`npm --prefix web run dev`); the Go
image does not serve the SPA. Port-forward: `NAMESPACE=terraflow-local bash scripts/portforward.sh 8080` (the
script defaults to namespace `terraflow`).

**BUG-1, 2026-11-13: not reproduced.** The cluster still ran `terraflow:v0.3.0-local` with nothing listening on
`localhost:8080` (no port-forward). With an image built from master (migrations 15 to 19 applied) and
`NAMESPACE=terraflow-local bash scripts/portforward.sh 8080`, opening "New from GitHub" as a dev user gives
`GET /api/github/status` 200 and `GET /api/github/repos` 200. This fits hypothesis 1 (something other than the new
pod answered on port 8080). The cluster database holds no GitHub connection, so the next run starts at Connect GitHub.

**BUG-3, 2026-11-13: Connect GitHub ends with `error=code`. Cause found, not yet fixed in the cluster.** GitHub
accepts the authorization and returns to `/api/github/callback`, then refuses the code exchange (server log:
`github callback failed code=code err="GitHub refused the authorization code"`). The `client-secret` in the cluster
Secret `tf-github` is not the App's current client secret: it is 15 characters long, the one in `.env.local` is 40,
and their fingerprints differ (the private key and the client id are the same in both). Fix: put the current client
secret into `tf-github` and restart the deployment, then repeat the run from Connect GitHub. Also noted:
`.env.local` has `TF_GITHUB_REDIRECT_URI=http://localhost:5173/api/github/callback` while the cluster uses
`http://localhost:8080/api/github/callback`; GitHub accepted the cluster's one, so it is registered on the App.

**Run of 2026-11-13 with the server started from `.env.local` (not the cluster):** `go run ./server/cmd/server` with
the Entra settings of `.env` blanked and a fresh database `terraflow_exit` on the compose Postgres (the database
`terraflow` there is at goose version 17 but already has the `providers` table, so migration 018 fails on it; left
untouched). Connect GitHub as the dev user Alice succeeded with the real App: one connection (`Anmewzaa`, active) and
97 repository grants, `Anmewzaa/demo-terraform` among them, and the repository picker lists them. Stopped there:
**the exit test of M1 then passed on real GitHub**: new folder `terraform/dev` in `Anmewzaa/demo-terraform`, a resource
group dragged onto the canvas, Save, Publish, Approve as owner, and the branch
`terraflow/demo-terraform/dev-b4681d7f` (one commit "Version 1", bot `mewstellaflow`, file
`terraform/dev/common/rg.tf`) exists on GitHub, one commit ahead of `master`.
**BUG-4 (found and fixed in this run):** the review screen failed with `repository, branch or folder not found:
/repos/.../git/trees/?recursive=1`. `GET /git/commits/{sha}` returns the tree at the top level (`tree.sha`), the code
read `commit.tree.sha` (the shape of `GET /commits/{ref}`), so the base tree was empty; the fake GitHub had the wrong
shape too, so tests passed. Fixed in `Source.commit` and in the fake (`githubtest`).
`TestLiveGitHub` passed the same day (created `terraflow/live-smoke-1794582482`). **BUG-5 (UI, fixed, not re-checked
in a browser):** after Save the notice banner took the 1fr row of `.tf-main-area` and pushed the Changesets view to
the bottom; the top bar and the notice now share one grid row (`App.tsx`).
Still to do: sign in with GitHub (M2) against real GitHub; the branches `dev-b4681d7f` and `live-smoke-*` can be
deleted on GitHub.

**BUG-1 (as first reported): "Request failed (404)" in the "New from GitHub" dialog. Not diagnosed.** `errorOf` now adds the request path (`Request failed (404) /api/github/repos`), so the next report says which call failed.

**BUG-2 (found and fixed): the return from GitHub never reopened the dialog.** `App` clears the URL hash on its first render, before `HomePage` mounts, so `#/projects/new?connected=1` and `?error=...` were lost. The hash is now read when `HomePage` is loaded (covered by an e2e test).
- Known: the text is the fallback of `errorOf` in `web/src/hooks/useGitHub.ts`, used only when the answer is not
  JSON with an `error` field. Every 404 the Go handlers write has a message ("connection not found", "repository
  not found", ...), so this one is most likely a plain `404 page not found` (a route that does not exist on
  whatever answered) or a 404 from the dev server. It is not yet known which request failed (`/api/github/status`,
  `/repos` or `/folders`).
- Hypotheses, most likely first:
  1. Something other than the new pod owns `localhost:8080` (an older `kubectl port-forward` to the old namespace
     `terraflow`, started by `portforward.sh` with its default namespace, or an older image with no `/api/github/*`
     routes). Check: `netstat -ano | findstr :8080`, then `Get-Process` for each PID, and
     `curl http://localhost:8080/api/github/status` (unauthenticated: `401` means the new routes exist, `404`
     means an old image or another server).
  2. The routes are registered only when the database is used (`cfg.Queries != nil && !cfg.NoDB` in `router.go`);
     confirm the pod does not run with `TF_NO_DB`.
  3. The browser is not using the Vite dev server, or Vite proxies to another port (`TF_API_TARGET`).
- To do: reproduce with the browser's Network tab open, note the failing URL and its response body, and write the
  cause here. Make `errorOf` include the request path for a non-JSON error, so the next report says which call failed.

**Checks on the GitHub side before the next run** (App `mewstellaflow`): the callback URL is exactly the one above;
the App is installed on `demo-terraform` with Contents read and write and Metadata read; "Request user
authorization (OAuth) during installation" is on; the client secret in `tf-github` is the current one.

**Next run, in order:** (1) fix BUG-1; (2) sign in with a dev user, open "New from GitHub", Connect GitHub,
authorize; (3) expect `/#/projects/new?connected=N` and the repository in the list (`error=no_installation` means
the App is not installed for that account); (4) pick the folder with `.tf` files and create the project; (5) open
the canvas (reads through the App), publish a change, approve as owner, expect a branch
`terraflow/<project>/<env>-<id8>` on GitHub; (6) record each failure here as a numbered bug.

**Also pending from that session:** local login and local projects must be dev-only (default `TF_DEV_LOGIN` off,
and refuse it beside a production setting); the live smoke test (`TestLiveGitHub`, written, not run). Committed since:
the chart change (`github.existingSecret`, init container `-U`), `.dockerignore`, the SHA cache, the database rate
limiter, catalog `source`, Playwright tests for the GitHub screens, and M2. Helm gotcha: `--reuse-values` turns a numeric `--set` value into a float (`5.682033e+06`), so set the App
ID with `--set-string`.

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

- **Built 2026-11-13 (D-060).** Scopes, the project catalog endpoints, the schema editor (live check, form preview, diff, export and import, draft from `variables.tf`) and the exit tests (`TestProjectCatalogBelongsToItsProject`, `TestASchemaForAnotherProviderValidatesAndIsListed`, `otherProvider.test.tsx`). Open: the agreed list of more `azurerm` resources.
- **Plan notes (2026-11-10, before the work).** What exists already: the admin import validates a schema and
  reports a diff with breaking changes (`catalog.Validate`, `ImportCheck`), and raw resource templates accept any
  provider. What is missing: (1) a scope on catalog modules and raw resources (`global` or a project id), which
  changes the unique key `(name, version)`, the catalog `Store` for the file and Postgres implementations, the
  list and get handlers (they must merge global and project modules, and a project module must never shadow a
  global one silently), the canvas palette and the generator's lookup by name; (2) owner and co-owner endpoints
  for a project's catalog; (3) the schema editor page (live validation, form preview, diff, export and import).
  The first step should be a design note with the scope rules, because the generator depends on module names.

### 8.7 to 8.16 Backend validation, environments, links, checks, templates (plan v3, 2026-11-14; nothing built yet)

Replaces the M5 to M9 draft of 2026-11-13. Numbers continue the Phase 8 sub-phases (8.0 to 8.6 are already used above).
Hard rules still hold: no Azure calls, schema-driven forms, module calls by default, honest state, stop at the
branch, never bypass branch policy. Every step ends with a decision entry (D-065 onward), tests and docs; the owner
checks the screens.

| No. | Name | Owner items | Size | Depends on |
|---|---|---|---|---|
| **8.7** | Validation moves to the backend | 3 (checks) | medium | nothing |
| **8.8** | Settings screen and the base branch | settings logo, free branch | small | nothing |
| **8.9** | Environments and a flexible layout | environments, any folder layout | large, riskiest | 8.7 |
| **8.10** | Compare two environments | env diff | medium | 8.9 |
| **8.11** | Links: private endpoint, NSG, route table | arrows | medium | 8.7 |
| **8.12** | Network modules | route table, DNS, NSG... | medium, waits on the modules repository | 8.11 |
| **8.13** | Plan-ready checks (policy rules) | checks | medium | 8.7, 8.9 |
| **8.14** | Repository drift | drift view | medium | 8.9 |
| **8.15** | Stack templates (project) | create template | large | 8.7, 8.9 |
| **8.16** | Shared template gallery on Home | public and private sharing | large | 8.15 |

Order: 8.7, 8.8, 8.9, then 8.10 and 8.11 in either order, 8.12, 8.13, 8.14, 8.15, 8.16. 8.9 is first among the big
ones because 8.10, 8.13, 8.14 and 8.15 all ask "which environment, which folder".

**Facts from the code that shape the plan (checked 2026-11-14)**
1. **The server does not validate a canvas.** Save draft, save version and publish store whatever the browser
   sends. Required fields and "a subnet needs a VNet" live only in `web/src/lib/validate.ts`; the server's only
   gate is that the generated HCL parses (and raw resource types, D-040). There is no JSON Schema validator in
   `go.mod`; `catalog.Validate` checks that a schema is well formed, not that a value fits it.
2. **The code knows one layout:** one folder per stack, and `env/<env>.tfvars` inside it (the parser builds the
   graph from the project's folders; the generator writes each node's `target_folder` as given). The owner's real
   repositories use `AIC/vnet/dev`, `AIC/dev/vnet`, `AIC/dev`, or one `main.tf`; `per_environment` is listed as not
   built in D-057. Node ids are `<folder>.module.<label>`, so a folder name that contains the environment changes
   every id.
3. **Arrows already exist for references.** `draftEdges.ts` draws an edge for each `module.<name>` in a draft's
   values; the `pick-existing` widget writes such a reference (it names one module type). The private endpoint's
   `target_resource_id` is a plain text box today, so nothing links it. The endpoint is nested in its subnet
   (`x-tf-canvas.parents`); its link to the target is a second relation.
4. **A subnet is not a module.** It is an entry of the VNet module's `subnets` map; the VNet schema has no NSG or
   route table field, and the private endpoint schema has no DNS zone argument. The modules live in the team's other
   repository.
5. **"Template" already means raw resource templates** (D-041, the Templates tab). The new feature is called a
   **stack template**.
6. **The branch is one value per project** (`project_repos.default_branch`); every read and `CreateBranch` uses it, and
   a changeset records its base commit (D-051).
7. The catalog admin can draft a schema from a pasted `variables.tf` (D-060): the way to add modules cheaply.

#### 8.7 Validation moves to the backend
**Status 2026-11-14: built (D-065).** `server/internal/validate`; `POST /projects/{id}/validate`; save refuses
unsafe input; publish and approve run every rule; the node panel shows what the server says (asked 0.6 s after the
last edit, `useServerFindings`) and a refused publish lists the problems; the catalog version of an imported block is
chosen by the server (`catalog.ResolveVersion`, the browser keeps its rule only for an older server); the nesting rule
is enforced by the engine (the browser keeps the drag hint). Tests: validate package (rules, hostile input, limits,
determinism, a 15 s fuzz run of about 4 million inputs without a panic), API guards on save, publish and the
validate endpoint, version resolution, and 115 web tests. **Not run:** the tests that need Postgres (Docker Desktop
would not start during this session) and any screen in a browser. **Left for later steps:** a badge per node on the
canvas, the audit rows marked Move that are display-only today, limits per user.

Goal (owner): the backend validates every stage; the browser computes less; hostile or broken input is stopped at the
server.
- **One engine.** `server/internal/validate` (pure: no I/O, no Terraform binary, no Azure): input is a canvas, the
  catalog entries it uses (the project sees global and its own, D-060) and the project settings; output is findings
  `{severity: error|warning|info, node, field, rule, message}`. Honest state: a value that cannot be evaluated is
  reported as unknown, never passed or failed (rule 8).
- **Stages.** (1) *Save draft and save version:* structural errors are refused with `422` (malformed JSON, size
  and count limits, a name or path that is unsafe, a type the schema forbids); incomplete work is **saved and
  flagged** (a draft may be unfinished). (2) *Publish:* errors block; warnings do not. (3) *Approve:* the same
  engine runs again against the repository at the base commit, so what was valid when published and what is valid
  against a repository that moved are both checked. (4) *Preview:* returns the findings next to the HCL.
- **Rules in the first set.** Value fits its schema (type, enum, pattern, min and max, items); required arguments;
  parent required and allowed (`x-tf-canvas.parents`), no containment cycle; the module and version exist in the
  catalog the project sees; names and file names are safe; folder is inside the layout (8.9); a reference
  `module.<x>` points at something on the canvas or in the repository; `{"$expr": ...}` is only a reference or a
  function call the generator knows, never free text that could break out of the HCL; node and size limits (for
  example 500 nodes, 256 KiB per value) so one request cannot exhaust memory. Duplicate names in a folder are found
  here, early, instead of only at merge.
- **Browser side.** The browser asks the server (`POST /projects/{id}/validate`, debounced, no side effects) and
  shows the answer, so there is one source of truth. Cheap local checks stay only where waiting for the server would
  feel broken (an empty required field turning red while typing); they are generated from the same schema and the
  server result wins when they disagree.
- **Frontend logic audit.** What exists in `web/src/lib` and where it belongs:

| File | What it computes | Decision |
|---|---|---|
| `validate.ts` `nodeIssues` | required fields, parent required | **Move** to the engine; keep an instant local copy |
| `validate.ts` `defaultValues` | defaults of a new node | Server returns them with the module; the engine applies them on save |
| `rawResource.ts` `rawIssues`, regexes | type and name rules | Server already checks the type; **add** name and folder rules to the engine |
| `canvasLayout.ts` `canNest`, `resolveParents` | which node may sit in which | **Move** the rule to the engine (the browser keeps the drag hint) |
| `importExisting.ts` `resolveCatalogVersion` | which catalog version a repository source maps to | **Move** to the server (the import response carries it) |
| `generation.ts` `suggestToken` | suggests `{org}` for a typed value | Keep (suggestion only); the server already substitutes variables |
| `provider.ts` `compatible`, `fitToProject` | which raw resources fit a provider version | Keep for choosing; **enforce** in the engine at publish (D-058 said "not a guarantee") |
| `draftEdges.ts` | arrows from references | Keep (display); the engine checks the reference is real |
| `diff.ts`, `hclBlocks.ts`, `valueText.ts`, `palette.ts`, `schemaEditor.ts` | display, filtering, file export | Keep (no trust decision); the server validates what is saved |

- **Dependency decision.** A JSON Schema validator is needed (recommended: a maintained Go library, for example
  `santhosh-tekuri/jsonschema`, over a hand-written subset that would drift from the schemas). A new dependency is a
  supply chain decision; the owner approves it.
- **Compatibility.** Additive API (`POST .../validate`; save and publish gain `422` bodies with `findings`).
  Existing drafts load as before; they are checked on the next save. No migration. Golden files unchanged.
- **Tests.** One case per rule; the same canvas gives the same findings in the same order; unknown stays unknown;
  publish and approve refuse from the API (not only the UI) when a rule fails; oversized and hostile payloads (a
  `$expr` that closes the block, a path with `..`, 100,000 nodes) are refused early; a fuzz test on the value
  checker.

#### 8.8 Settings screen and the base branch
- **Screen.** The rail item "Project" (users icon) becomes **Settings** with a gear icon. Inside, two tabs: **Project**
  (people, repository, second reviewer, transfer, delete) and **Settings** (variables, providers, environments,
  layout, starter files, branch). The tab is in the URL hash. This replaces D-059 (2).
- **Branch.** `project_repos.base_branch` (migration 021; empty means the repository default, so nothing changes for
  existing projects). Reads and the starting point of the review branch use it. Chosen from the repository's real
  branches (installation token, `GET /repos/{o}/{r}/branches`, paged and capped; owner and co-owner only) or typed;
  saving checks it exists and is not a `terraflow/` review branch. It is only **read**; Approve still creates a new
  `terraflow/<project>/<env>-<id8>` branch and never commits to the base (rules 1 and 7). A base branch deleted later
  puts the project in "needs attention" (like D-061); no silent fallback. Pending changesets keep their recorded
  base commit; the screen says how many were made on the old base. One branch per project, not per environment:
  environments are folders or files of one tree.
- **Tests.** Fake GitHub with several branches: read from a chosen branch, review branch created from it, missing
  branch, refused names, only owner and co-owner may change it.

#### 8.9 Environments and a flexible layout
Goal (owner): work with any repository shape, and set it up in a minute.
- **Environments are a project setting.** A list such as `sbx, int, dev, test, stage, prod` (names follow
  `ValidEnv`: letters, digits, `-`, `_`), chosen at "New project" (Advanced step) or later in Settings, with
  presets (`dev, prod`; `dev, test, prod`; `sbx, int, dev, test, stage, prod`) and free text. The canvas switch shows
  this list instead of the fixed `dev, staging, prod`.
- **The layout is two small settings, not a fixed shape.**
  1. **Folder pattern**, from tokens `{stack}` (the module's folder, such as `vnet`) and `{env}`:
     `{stack}/{env}` gives `AIC/vnet/dev`; `{env}/{stack}` gives `AIC/dev/vnet`; `{env}` gives `AIC/dev` (one folder
     per environment); `{stack}` gives `AIC/vnet` (today's layout); `.` gives one folder for everything.
  2. **Values pattern** (where an environment's values live): `{dir}/env/{env}.tfvars` (today's default),
     `{dir}/{env}.tfvars`, `{dir}/terraform.tfvars`, or none.
  The **file mode** that already exists (D-057: one file per module such as `rg.tf`, or everything in one `main.tf`)
  stays a third, independent choice. So "one `main.tf`" is not a special case: it is pattern `{env}` or `.` with file
  mode `single`. When the pattern has no `{env}`, the environment is a values file; when it has `{env}`, the
  environment is a folder.
- **Easy setup.** (a) *An existing repository is detected:* the server scans the folder tree, looks for the
  environment names among the folder names (or `*.tfvars` names) and proposes the pattern and values pattern, with
  the tree it found shown beside what it understood ("`vnet/dev` = stack `vnet`, environment `dev`"). The owner
  accepts or edits. (b) *A new or empty repository:* five presets with a live preview of the paths a VNet and a
  storage account would get. (c) *Advanced:* type the patterns; invalid ones (unknown token, `..`, absolute path,
  two `{env}`) are refused with the reason. A "check against the repository" button lists folders that do not fit.
- **Server shape.** One resolver, `layout.Resolve`, maps a path to (stack, environment, file) and back; the parser,
  importer, generator, merge layer and the checks all go through it, so no other code knows a pattern. The default
  settings (pattern `{stack}`, values `{dir}/env/{env}.tfvars`) are exactly today's behavior, so every existing
  golden and round-trip test must pass unchanged before anything new is added. Node identity gets a layout-free key
  `(stack, label)` next to the id, so the same resource can be paired across environments (8.10) and across layouts.
- **Parser and generator.** The graph for environment E reads the folders the pattern maps to E, evaluates with E's
  values and keeps data-source name matching inside E. A new resource is written to the folder the pattern gives for
  its stack and the environment on the canvas; the merge rules (D-036, D-064) are unchanged.
- **Risk.** Highest of Phase 8: the parser, the generator and node identity change. Mitigation: the resolver first
  with today's layout and all existing tests untouched; then the other patterns, each with fixtures in `samples/`
  (synthetic, never real names): `{stack}/{env}`, `{env}/{stack}`, `{env}` with several files, `{env}` with one
  `main.tf`, and `.`; round-trip on each.
- **Compatibility.** Settings JSON gains `environments`, `layout.pattern`, `layout.values` (no migration; empty means
  as before). OpenAPI additive. Generated HCL for existing projects unchanged.
- **Tests.** Resolver table tests (path to tuple and back, rejected patterns); the five fixtures; detection on
  each fixture proposes the right patterns; two environments evaluate with their own values.

#### 8.10 Compare two environments
- **Compare.** Pick two environments; the server evaluates both (defaults, locals and each environment's values)
  and pairs resources by `(stack, label)`: same, differs (argument by argument), only in one. It also lists
  variables that have a value in one environment and none in the other. An unresolved value compares as unknown,
  never as equal. Read-only `GET /projects/{id}/env-diff?from=&to=`; the screen is two columns with changed
  arguments highlighted, filters (only differences, one stack) and a summary count.
- **Copy to another environment.** For one resource or one argument: writes the block into the other
  environment's folder (or the value into its values file) as a draft change, named for that environment by the
  naming rule the project uses; it goes through the normal validate, review and approve path. Collisions are refused
  as in D-036.
- **Tests.** Fixtures with two environments; unknown handling; a resource only in `prod`; copy produces the expected
  block and refuses a collision.

#### 8.11 Links: private endpoint, NSG, route table
- **Metadata, not names (rule 4).** `x-tf-link` on a schema: the target advertises what it offers (`offers`:
  kind `private_link`, `subresource`, `output`), the source says what it accepts and into which field. The
  `pick-existing` widget gains `link` (a kind) next to `module_type`, offers only compatible nodes, writes
  `module.<target>.<output>` and fills dependent fields (the endpoint's `subresource_names` from the offer).
- **Private endpoint.** Stays **inside its subnet** and carries two links: to the real resource it connects (key vault,
  storage, database) and to the private DNS zone. Dragging from its link handle to a compatible node sets the
  argument; an incompatible node is not offered.
- **NSG and route table on a subnet.** They are links, not containment (one NSG serves many subnets). Shown three
  ways at once: a dashed link, a **chip** on the subnet's header (icon and name), and the subnet's Relationships tab
  with a change action. Until the VNet module takes these inputs, association modules (8.12) are what the link edits.
- **Drawing.** `referenceEdges` already draws an edge for the reference; a reference in a field that has `link` is
  drawn dashed and labelled ("private link", "NSG"), from the source to its target. An imported endpoint
  (`target_resource_id = module.kv.id`) shows the same arrow, because the canvas decides the style from the schema.
  The graph contract and golden files do not change.
- **Tests.** Schema validation of `x-tf-link`; a link to an incompatible target is refused by the engine (8.7);
  generator golden for an endpoint linked to a key vault; the same arrow from an imported endpoint.

#### 8.12 Network modules
- **To agree with the owner:** route-table (with routes), public-ip, nat-gateway, private-dns-zone virtual network
  link, NSG and route table association modules, and the targets that can offer a private link (key-vault,
  storage-account, postgresql-flexible-server, mssql-server, cosmosdb-account, redis-cache, servicebus-namespace,
  container-registry). The list should be the 20 modules of PR #6 of the modules repository.
- **How.** Schemas are drafted from each module's `variables.tf` (D-060), reviewed, and seeded. A module can only be
  offered if it takes the inputs and gives an `id` output; the private endpoint module needs
  `private_dns_zone_ids`, and the VNet module needs NSG and route table inputs (or the association modules are
  used). These changes are in the modules repository, not here.

#### 8.13 Plan-ready checks (policy rules)
On top of the 8.7 engine, rules that are the team's policy, all findings (warnings unless the project makes them
errors): a naming pattern per resource kind, kinds that need a lock in `prod`, allowed regions, required tags, a
variable used in an environment with no value, an environment that differs from another in a way the project marked
as forbidden. Schema rules `x-tf-checks` (required-with, mutually exclusive, pattern, allowed values) let a module
carry its own. Settings has a Checks section; the canvas has a Problems list and a badge per node; the publish
dialog lists findings. Tests: one per rule, deterministic order, unknown stays unknown.

#### 8.14 Repository drift
Hard rule 2 forbids asking Azure, so this is **not** infrastructure drift: it is the repository against what
TerraFlow last wrote. (a) blocks on the base branch that the canvas does not show or cannot represent (`dynamic`,
labelled nested blocks); (b) files changed on the base branch since the last approved changeset; (c) values that
differ from the project variables. A Drift list and the palette's "In repository" tab; each item can be imported
into the canvas (D-036) or dismissed with a note kept per project. Tests: fixtures where a commit lands after the
last approval.

#### 8.15 Stack templates (project)
- **Create.** Select nodes (marquee or shift-click), "Create stack template": the nodes, the arrows between them and
  **slots** for everything that pointed outside the selection (the parent VNet, a DNS zone). Module nodes are pinned
  to the version used; raw resources are allowed.
- **Parameters.** An argument can be marked a parameter (name, type, default, a value per environment). A
  parameter becomes `var.<name>` in the module call, a `variable` where missing, and a line in each chosen
  environment's values file through the D-057 merge rules (append or edit one line, never rewrite). A secret is never
  a parameter. Parameters become project variables, so one place owns the values.
- **Apply.** Choose a template, one or more environments, fill slots and parameters in a form rendered from the
  template; the nodes appear as a draft in each environment's folder (8.9), names made unique with a prefix or
  suffix the person chooses; label collisions refused. Everything goes through the 8.7 engine.
- **Storage.** `stack_templates` (migration 022): project, name, version, canvas JSON, parameters, slots.
- **Open.** Upgrade offers when a pinned module has a newer version; what happens when a module in a template is
  retired (D-037).

#### 8.16 Shared template gallery on Home
- **Scopes.** A template is **private** (its author), **project** (the project's members) or **public** (every
  signed-in person, shown on the Home page in a Templates section with search, category, author, a preview of the
  diagram, the modules it uses, and an Import button). Public means public to signed-in users of this TerraFlow,
  not to the internet.
- **Export and import.** A template exports as a JSON file and imports into any project. Importing is untrusted
  input: it passes the 8.7 engine (schema, names, size, `$expr`), modules must exist in the importing project's
  catalog (otherwise the import says which are missing and offers nothing that would run code), and the result is
  a draft, never written to the repository without review.
- **Publishing public.** Recommended: a person submits, an admin approves (like sign-up approval, D-055), because a
  public template reaches everyone and carries module sources and arguments. Before submit the server scans for
  things that look like secrets, real subscription ids or key material and refuses or warns; defaults are kept,
  per-environment values are not exported unless marked.
- **Management.** Author edits and versions, withdraws, forks; admin can hide or remove; audit for publish, import,
  withdraw; an import count and "used in N projects" on the card.
- **Tests.** Visibility per scope (private is invisible to others, public needs sign-in), a template with a missing
  module, a hostile template, an approval flow, secret scan cases.

**Answers of the owner, 2026-11-14.** (1) Use an outside library wherever it does the job better than new code
(the JSON Schema validator is the first). (2) Public templates need an admin's approval before they appear, so
**8.16 includes an admin Review templates page**; public templates are visible only to signed-in people; a private
template is visible to its author and to admins only. (3) The dashed link is accepted; the canvas must avoid UI bugs
and must keep a dashed line from running over a resource (8.11: route around nodes, draw links under nodes, and
test the layout with crowded canvases). (4) The owner will add the modules once given a list (8.12). Everything
should be safe and flexible for later features, and the screens easy to understand (every new screen has a plain
explanation line and a preview before it changes anything).

**Open decisions for the owner (my recommendation first):** (1) the layout model of 8.9, two patterns plus the
existing file mode, with detection and presets; (2) the JSON Schema library for 8.7; (3) drafts are saved with
findings, only malformed or unsafe input is refused; (4) public templates need admin approval and are visible to
signed-in users only; (5) base branch is one per project; (6) the module list of 8.12 from PR #6 of the modules
repository; (7) the arrow runs from the endpoint to its target, drawn dashed.

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
- **Anyone may try to sign in.** The default sign-up mode is approval (above); limits per user
  (projects, requests) still need a decision before go live, mainly for the `open` mode.
- A contributor acts through the owner's connection; if the owner later loses access to the
  repository, TerraFlow only notices when the owner signs in with GitHub or re-verifies.
- GitHub API rate limits per installation; reads are cached by commit SHA.
- Admin visibility of users' projects is a privacy decision (D-050 sets the default).

---

## Phase 9 — Edit existing resources (post-GA)
Modify existing/hand-written resources. Highest merge risk, intentionally after
go-live.

**Status 2026-11-13.** Most of this was built in Phase 6 (D-036, D-040). First step done (D-064): comments
between the arguments of an edited block are kept (`server/internal/hcl/comments.go`, tests in `comments_test.go`;
golden and round-trip tests unchanged). Left, to agree before work: `/* */` and nested-block comments; delete or
rename an existing resource with a check of the references to it; hand-written resources with `dynamic` or
labelled nested blocks.

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
