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
- **T:** golden tests — `samples/iac/project-1` → expected graph JSON (incl.
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

## Phase 5 — Review + branch + audit
Close the loop.

- **BE review:** list pending; build the **canvas diff** + **HCL diff** (vs
  `main`); **Approve** → generate + create a **branch** on ADO (Pushes API, one
  commit, branch policy untouched); **Reject** + required reason → back to the
  developer; **audit log** (who/what/when) table + migration.
- Optional per scope: enforce **separation of duties** (requester ≠ approver).
- **FE:** review screen (canvas diff + Monaco HCL diff), Approve / Reject(reason),
  status tracking, audit view.
- **T:** end-to-end against a **test ADO repo** — publish → review → approve →
  branch exists; reject → returns to developer; audit rows written.
- **Exit:** the whole flow works end-to-end on the test repo.

---

## Phase 6 — Hardening (production readiness)
- Security review (threat model, secret-handling checklist, branch-policy/identity
  scope audit), RBAC edge cases, error/empty/loading states everywhere.
- Performance on large repos and ~80+ node canvases (parse time, layout, render).
- Accessibility pass (focus, keyboard, reduced-motion, contrast — per design
  system).
- **User manual for DevOps**, final docs sweep, backlog cleanup.
- **Exit:** ready for production use.

---

## Phase 7 — Edit existing resources (post-GA, not in v1)
Modify existing/hand-written resources. Highest merge risk — intentionally after
go-live.

---

## Sequencing & risk
- **Critical path:** P0 → P1 (HCL engine) → P2 → P3 → P4 (generator) → P5 → P6.
- **Parallelizable:** access/provisioning requests during P0–P1; Helm/deploy
  (P2 INF) alongside P2 BE; the UI-design conversation during P3 so P4 can start
  clean.
- **Biggest risks, proven early:** (1) evaluating `var`/`locals`/`.tfvars` to
  resolve names in P1; (2) generating HCL that matches the team's style
  byte-for-byte in P4. Both are guarded by golden + round-trip tests.
- **Needs a real sample before coding:** confirm the 9 conventions in
  `samples/README.md` against the real repo (DevOps) before P1.
