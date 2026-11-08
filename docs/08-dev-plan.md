# 08 — Development plan

Task-level breakdown of the phases in `docs/03-phases.md`. Principles unchanged:
every phase is demoable, read-only before write, and the riskiest piece (HCL
parse/evaluate/generate) is proven first.

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

- **Decision at phase start:** how modules are added — admin-registered schema
  vs auto-extract from `variables.tf` + a UI-metadata overlay. Record as a new
  `D-xxx`.
- **BE `internal/catalog`:** module registry (module, version, source, schema,
  UI metadata, layout mapping, allowed-per-project); `variables.tf` → **JSON
  Schema** (types, defaults, `validation`, descriptions) + UI overlay (labels,
  dropdowns, hidden/derived).
- **BE API:** list catalog for a project, get a module's schema.
- **FE:** catalog panel; render forms from JSON Schema with
  react-jsonschema-form + custom widgets (pick-existing-resource dropdown fed by
  the inventory, locked/derived fields).
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
  Publish; live **Monaco** preview of the generated HCL.
- **T:** **golden-file** (canvas JSON → `.tf`, byte-for-byte) and **round-trip**
  (parse existing → add one module → write; untouched bytes incl. comments
  unchanged). These are the backbone — never weakened to pass a diff.
- **Exit:** DevOps signs off that generated `.tf` matches what they would write
  by hand on the sample repo.

---

## Phase 5 — Review + branch + audit (local) — DONE 2026-11-08
Review screen (canvas + HCL diff), Approve → branch in a local git repo
(`internal/localrepo`), Reject with required reason, audit log, optional
separation of duties (D-026, D-027). Real ADO deferred to Phase 6.

---

## Phase 6 — Features + UI v2 (major, local only)
No ADO read/push. Everything runs against `samples/` and the local repo.
Order: **6.0 -> 6.3 API -> 6.1 -> 6.2 -> 6.3 UI** (Home and Admin share one API).

- **6.0 Safe generator + samples (do first; generator part DONE, D-036).** Bug seen on `common/rg.tf`: a new
  `resource-group` node defaulted to file `rg.tf`, and the generator wrote the whole
  file, so the three existing modules (`rg_network`, `rg_app`, `rg_data`) were
  deleted. The generator must **never replace existing content**: parse the existing
  file, append the new block with `hclwrite`, keep untouched bytes, and fail (not
  overwrite) on a duplicate module label. Preview/diff shows only additions.
  `samples/` reworked into the mock projects **AIC** and **AIH** (DONE): AIC =
  one file per resource kind, five folders; AIH = everything in `main.tf`, two
  folders. Goldens: `aic-dev`, `aic-prod`, `aih-dev`. The graph builder was made
  deterministic (it listed edges in map order). **T:** regression golden for the bug above,
  round-trip with an existing multi-module file, duplicate-label rejection.
- **6.1 Existing resources are editable.** Imported nodes carry `existing`; editing and publishing them shows in the Approver mock (D-036).
- **6.1 Home.** Create / edit / delete projects stored **only in Postgres**.
  Delete removes the project from the app only; repo files are untouched. A path
  such as `AIC/` can be (re-)imported as a new project, and its existing `.tf`
  becomes canvas components. App-side config (e.g. block colour) lives in the DB.
- **6.2 Drag and drop.**
  - 6.2.1 Left panel lists Terraform **resources** and **modules** with a search box.
  - 6.2.2 Main board: nicer nodes and colours, nested resources, **arrows between
    resources** (private endpoint -> storage account) including across resource
    groups. The UI draws the link; the generator decides the HCL (same-folder
    reference or a `data` block across folders, D-033).
  - 6.2.3 Left panel shows the Terraform config when creating/editing; HCL Preview
    lives only in the left nav (remove the duplicate entry).
  - 6.2.4 HCL Preview page keeps the folder tree + code view; wrong output fixed by 6.0.
  - 6.2.5 Approver page: UI kept as a mock; Phase 5 backend stays unchanged.
- **6.3 API + DB (S3, DONE, D-037):** users and project members, project CRUD, component settings, module import with strict validation, publish/deprecate; Admin and project-route guards; migrations 008-010; OpenAPI 0.5.0. The Admin **pages** below are still to do.
- **6.3 Admin area (new pages).** Manage resource types, modules (import from
  schema), projects and users. **Schema import** with strict validation (D-035).
  Project visibility for users is assigned in the app; Entra only decides
  Admin vs User (D-032). Both component kinds (module call, raw `azurerm_*`
  resource) come from the same schema mechanism (D-033).
- **Contract:** all new endpoints in `api/openapi.yaml`; new migrations only.
- **Exit:** see `docs/03-phases.md` (Phase 6).

---

## Phase 7 — Security improvements
Threat model, secret-handling checklist, identity-scope audit, RBAC edge cases
(incl. new in-app project assignment), error/empty/loading states, perf on 80+
nodes, accessibility, DevOps user manual. **Exit:** ready for production use.

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
