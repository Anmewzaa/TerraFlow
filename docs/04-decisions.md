# 04 — Decision log

Short records of decisions and the reasoning behind them. Add new entries at the
bottom. Do not rewrite old entries. To change a decision, add a new entry that
supersedes the old one.

---

## D-001 No `terraform plan` inside Terraflow

- **Date:** 2026-11-07
- **Decision:** The review screen shows a canvas diff and an HCL diff only. The real plan runs in the ADO PR pipeline.
- **Why:** A plan needs read access to Azure and to Terraform state. Terraflow must not hold Azure credentials and has no branch before approval.
- **Alternatives rejected:** creating a temporary preview branch and triggering a plan pipeline. This adds complexity and creates branches before approval.

## D-002 Two-stage approval

- **Date:** 2026-11-07
- **Decision:** DevOps approves in Terraflow before a branch is created. The normal ADO PR review happens afterwards, outside Terraflow.
- **Why:** This stops unreviewed requests from creating branches (branch spam). Terraflow does nothing after branch creation.

## D-003 PostgreSQL as Terraflow's database

- **Date:** 2026-11-07
- **Decision:** Drafts, review status, reject reasons and the audit trail are stored in Azure Database for PostgreSQL Flexible Server.
- **Why:** These records exist before any branch does, so Git cannot hold them. A managed server gives backup, HA and patching. The database may also be used for other features later.

## D-004 Inventory of existing resources comes from the repo, not Azure

- **Date:** 2026-11-07
- **Decision:** Terraflow parses `.tf` on the repo's main branch to list existing resources that can be referenced. It does not query Azure.
- **Why:**
  - It keeps the "no Azure access" rule.
  - It avoids leaking resources across projects through a broad Reader role.
  - It excludes click-ops resources that are not managed by Terraform.
  - It works in an air-gapped network.
  - A reference to something not yet applied is caught by the plan in the PR pipeline.
- **Cost:** names built from `var`/`locals`/`.tfvars` must be evaluated.
- **Future:** an optional existence check through Azure Resource Graph (Reader).

## D-005 Generate `data` blocks for cross-folder references

- **Date:** 2026-11-07
- **Decision:** When a new resource references a resource from another folder, Terraflow generates the `data` block in the team's style. Derived values such as the resource group are locked in the UI.
- **Why:** If DevOps still had to hand-write `.tf`, the project would fail its purpose.

## D-006 Reject requires a reason and returns to the developer

- **Date:** 2026-11-07
- **Decision:** Reject requires a reason. The request goes back to the developer, who can edit and publish it again.

## D-007 No notifications in v1

- **Date:** 2026-11-07
- **Decision:** No Teams or email notifications. Status is visible in the web UI only.

## D-008 Entra BFF with federated credential

- **Date:** 2026-11-07
- **Decision:** The backend handles OIDC and keeps a server-side session behind an httpOnly cookie. The app registration uses a federated credential bound to Workload Identity, so there is no client secret.
- **Why:** No tokens in the browser and no long-lived secrets to rotate. This fits bank security expectations.

## D-009 One Workload Identity for PostgreSQL, Key Vault and ADO

- **Date:** 2026-11-07
- **Decision:** A single user-assigned managed identity, with least privilege on each target.
- **Why:** One pod would hold every identity anyway, so splitting them would not reduce the blast radius. Real separation needs a separate branch-writer service. Revisit if security review asks for it.

## D-010 Group overage handling

- **Date:** 2026-11-07
- **Decision:** Prefer tokens that contain only groups assigned to the application. The backend detects the overage claim and returns a clear error. Microsoft Graph is not called.
- **Why:** Overage depends on how many groups each user is in, not on the number of app users. Calling Graph would need extra permissions and egress.

## D-011 Single container image

- **Date:** 2026-11-07
- **Decision:** Go embeds the built SPA and serves it from the same origin as the API.
- **Why:** One deployment. The same origin keeps the BFF cookie simple and removes the need for CORS.

## D-012 Edit of existing resources deferred to Phase 7

- **Date:** 2026-11-07
- **Decision:** v1 creates new resources and references existing ones, but does not modify existing or hand-written resources.
- **Why:** Merging with hand-written code is the highest-risk area. It should wait until after go-live.

## D-013 No CI pipeline for Terraflow in scope

- **Date:** 2026-11-07
- **Decision:** Building and publishing Terraflow is not part of this project's deliverables.

## D-014 Production infrastructure deferred to Phase 8

- **Date:** 2026-11-07
- **Decision:** Workload Identity, Azure PostgreSQL (Entra passwordless), Key Vault, ADO repo integration, and AKS deployment are moved to Phase 8. Phases 3–6 develop against local Postgres, local samples, and local k8s.
- **Why:** These are infra/ops concerns that do not block feature development (module catalog, canvas editing, review flow). Developing locally first keeps the feedback loop fast and removes the dependency on bank access approvals. Phase 8 upgrades from local to production when features are ready.

## D-015 Admin-registered module schemas

- **Date:** 2026-11-08
- **Decision:** Module schemas are authored by an admin as JSON files (JSON Schema + UI metadata) and seeded into Postgres via a migration/seed job. The API serves schemas from the database, not from disk at runtime.
- **Why:** Admin-authored schemas give full control over UI behavior — field types (dropdown vs text), hidden/derived fields, pick-existing-resource bindings, grouping, and ordering — without relying on auto-extraction from `variables.tf`, which cannot express these concerns. The database is the runtime source of truth so the API is fast and stateless; the JSON files in the repo are the authoring source of truth so schemas are version-controlled and reviewable.
- **Alternatives rejected:** Auto-extract from `variables.tf` — cannot express UI hints, derived fields, or layout mapping. Hybrid (auto-extract + overlay) — adds complexity for a small team that can maintain schemas directly.

## D-016 Local module source for Phases 3–6

- **Date:** 2026-11-08
- **Decision:** The module catalog reads from `samples/terraform-modules/` during local development. ADO module repo integration is deferred to Phase 8 alongside other production infrastructure.
- **Why:** Consistent with D-014. The sample modules are sufficient to build and test the catalog, forms, generator, and review flow.

## D-017 Global module catalog

- **Date:** 2026-11-08
- **Decision:** The catalog is global — every project sees every registered module. There is no per-project allow-list in v1.
- **Why:** The team uses a single shared module repo. Per-project scoping adds complexity with no current need. Can be added later as a filter without schema changes.

## D-018 Versioned module catalog

- **Date:** 2026-11-08
- **Decision:** Each module version is a separate catalog entry, stored under a directory convention like `storage-account/v1/`. The schema, UI metadata, and source ref are tied to a specific version. Multiple versions of the same module can coexist in the catalog.
- **Why:** Real modules are versioned with `?ref=vX.Y.Z`. A new version may add, remove, or change variables, so the schema must be version-specific. Developers pick the version when adding a module to the canvas.

## D-019 UI metadata lives inside the registered schema

- **Date:** 2026-11-08
- **Decision:** UI-specific metadata (field display type, hidden/derived markers, pick-existing-resource bindings, field grouping and ordering) is part of the registered JSON Schema file, not a separate overlay file.
- **Why:** One file per module version keeps things simple. The schema is admin-authored (D-015), so there is no auto-generated base to overlay.

## D-020 Layout mapping in module metadata

- **Date:** 2026-11-08
- **Decision:** Each module's registered metadata includes a `target_folder` field that tells the generator which project folder the module call belongs in (e.g. key-vault → `keyvault/`). This is used by the HCL generator in Phase 4.
- **Why:** The team's convention is one root module per folder. The mapping must be known at catalog time so the canvas can show folder placement and the generator can write to the correct file.

## D-021 Merged canvas graph with draft layer

- **Date:** 2026-11-08
- **Decision:** The editable canvas merges parsed (existing) resources and draft nodes into a single React Flow graph. Existing nodes are read-only and selectable for reference. Draft nodes carry `status: "draft"` and are styled distinctly (badge + visual treatment). There is no separate "draft canvas" — one graph, two layers of data.
- **Why:** A single graph lets the developer see context (existing resources) while adding new ones. Existing resources are already laid out by the parser; overlaying draft nodes avoids duplicating that work and makes cross-folder references intuitive.

## D-022 Auto-placement by target_folder

- **Date:** 2026-11-08
- **Decision:** When a module is dragged from the catalog onto the canvas, it is placed inside the folder group matching the module's `target_folder` metadata (D-020) automatically. The developer can reposition the node within that folder but cannot move it to a different folder.
- **Why:** Folder assignment is a structural decision driven by module metadata, not a layout choice. Auto-placement prevents miscategorization and keeps the generated HCL in the correct file. Free repositioning within the folder lets the developer organise visually.

## D-023 Pick existing resource via dropdown

- **Date:** 2026-11-08
- **Decision:** Schema fields that reference an existing resource (marked with `x-pick-resource` in the module schema) render as a dropdown populated from the parsed inventory. Selecting a resource locks its derived values (e.g. resource group, location) with the `.tf-locked` treatment.
- **Why:** A dropdown is simple, discoverable, and works at the field level without requiring the developer to hunt on the canvas. The inventory is already parsed; filtering it for the dropdown is straightforward.

## D-024 Per-canvas changeset draft

- **Date:** 2026-11-08
- **Decision:** A draft is a changeset: the full set of module instances, their form values, and edges that the developer has added in one editing session. Save persists the entire changeset. Publish submits the changeset for DevOps review.
- **Why:** A per-module draft (Phase 3) worked for exploring a single form. Phase 4 adds multiple modules with cross-references, so the unit of work is the changeset, not a single module. This also aligns with the review flow (D-002): the reviewer sees one coherent change, not scattered module forms.
- **Migration:** The existing per-module draft API continues to work for form auto-save; the changeset wraps and supersedes it for the publish flow.

## D-025 HCL preview before publish

- **Date:** 2026-11-08
- **Decision:** The developer can preview the generated HCL before publishing. The preview runs the same generator that will produce the final `.tf` files on approval. Approve (Phase 5) generates the final `.tf` and creates the branch.
- **Why:** Letting the developer see what will be generated catches mistakes early, before the DevOps reviewer is involved. The exit criterion for Phase 4 is that DevOps signs off on the generated output, so the preview is also the developer's own quality gate.

## D-026 Phase 5 runs against a local repo; Tailwind joins the web stack; projects are created in-app

- **Date:** 2026-11-08
- **Decision:** (1) In Phase 5 the Approve step writes the generated `.tf` and creates the branch in a **local git repo** instead of calling ADO; the real ADO Pushes API client is deferred. (2) The web app adopts **Tailwind CSS v4 (utilities only, no preflight)** alongside the Harbor tokens; new UI uses Tailwind classes that reference `--tf-*` tokens. (3) Projects are created from the Home screen and stored in Postgres; with no `iac_path` the server creates `<TF_WORKSPACE_DIR>/<slug>`, and an explicit path must lie inside the workspace (or beside the sample project).
- **Why:** No test ADO repo exists yet; a local branch exercises the same review flow without any external write (hard rules 1-3 still hold: the app stops at the branch). Skipping Tailwind's preflight avoids restyling existing Harbor components. Constraining `iac_path` stops the server from parsing arbitrary directories.
- **Scope note:** Phase 5 is split in two: part 1 = UI (Home, canvas panels, code view, backdrop); part 2 = review, approve to local branch, reject, audit.

## D-027 Local review branch: worktree commit, devops-only, audited

- **Date:** 2026-11-08
- **Decision:** Approve generates the `.tf` files and commits them on a new branch `terraflow/<project-slug>/<env>-<changeset-id8>` in the project's local git repo, using a temporary `git worktree` so the checked-out branch and working tree are never touched. A project folder inside the workspace that is not yet a repo is `git init`-ed on first approve. Approve/Reject require the `devops` role; `TF_SEPARATION_OF_DUTIES=true` also forbids reviewing your own changeset. Reject needs a reason. Publish, approve and reject each write an `audit_log` row (`detail.project_id` links it to the project). Approving is refused (409) when a generated file already exists, or the branch exists; it never overwrites.
- **Why:** Mirrors the future ADO Pushes API (one commit, new branch, no policy bypass) without any external write, and a local dev can try the full flow safely. The sample project sits inside the TerraFlow repo, so approving it adds a branch to that repo - use a workspace project for experiments.
- **Migration:** `006_changeset_branch.sql` adds `branch_name`, `commit_sha` and an audit index. The ADO client will replace `localrepo.CreateBranch` behind the same `BranchWriter` signature.

## D-028 Aurora palette (cyan on black) with a derived light mode

- **Date:** 2026-11-08
- **Decision:** The semantic colour tokens are re-defined in `web/src/design-system/aurora.css` (loaded after the Harbor tokens): dark mode is the Home page's black + cyan; light mode is derived from it (pale cyan-white surfaces, cyan-700 actions). The nav rail, top bar, React Flow controls, Home backdrop and the code viewer (VS Code Light+/Dark+ palettes) all follow the theme; the choice is saved in `localStorage` (`tf-theme`) and applied before first paint. Harbor shapes, spacing and type are unchanged.
- **Why:** One look across Home and the canvas, and a real light mode instead of an always-dark Home and an always-navy nav rail. Gold stays only as the "draft" accent. Supersedes the Sapphire & Gold colour choice in `docs/05-design-system.md`; update that doc when the palette settles.

## D-029 Schema-driven nesting, revisions and HCL diff colours

- **Date:** 2026-11-08
- **Decision:** (1) A module schema may carry an `x-tf-canvas` block: `container` (can hold resources), `parents` (module names it may sit in), `parent_required`, `nest.field` (the node is folded into the parent's input map instead of becoming a module call - a subnet becomes `subnets["snet-x"]` of the VNet module) and `parent_ref` (a field filled from the parent, written as `{"$expr": "module.{module}.subnet_ids[\"{parent_name}\"]"}` and resolved by the generator). The canvas node stores `parent_id`, `nest_field` and `color`. No UI code names a module. (2) Map keys that are not plain identifiers are quoted in generated HCL (`"snet-app" = {`), as in `samples/`. (3) Editing earlier work: **Revise** copies a published, approved or rejected changeset into a new draft whose `base_canvas_json` is the source canvas (migration 007). The preview returns `files` and `base_files`; the client diffs them line by line: green = added, yellow = modified (hover shows the old line), red = removed. A brand-new changeset has no base, so everything is green. (4) Removing a container removes everything inside it.
- **Why:** Subnets and private endpoints are not top-level resources, and reviewers need to see what an edit changes. Editing `.tf` files that already exist in the repo stays out of scope (Phase 7); the diff is against the previous saved version, not the repo.
- **Limits:** One level of folding (subnet in VNet); no re-parenting by dragging yet.

## D-030 Output file is configurable per resource

- **Date:** 2026-11-08
- **Decision:** Each canvas node carries `filename` (and `target_folder`), both editable in the right panel ("Output file"). A module schema sets the default with `x-tf-module.filename` (resource-group `rg.tf`, key-vault `kv.tf`, storage-account `storage.tf`, virtual-network `vnet.tf`, network-security-group `nsg.tf`, ...), matching the team's one-file-per-resource layout in `samples/`. An empty name means `main.tf`, so drafts saved before this change generate exactly as before. Resources that share a file name are written to the same file, and generated data blocks still go to `data.tf` (merged if a resource chose that name). Names must match `^[A-Za-z0-9][A-Za-z0-9_.-]*\.tf$` and folders are a single plain segment; the server enforces this, and the UI mirrors it.
- **Why:** Hard-coding `main.tf` made DevOps rename or move generated code by hand, which is what the generator exists to prevent (CLAUDE.md rule 5). Making it a schema default plus a per-node override keeps UI code free of module names (rule 4).
