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

## D-031 Roadmap after Phase 5: ADO integration before Hardening

- **Date:** 2026-11-08
- **Decision:** Phase 5 is closed. **The phase list is superseded by D-034.**
- **Why:** `internal/ado` is the only writer of external state and had not been exercised yet; its approvals take long at a bank. Proving it early and then hardening the real path is lower risk than hardening a local stand-in.

## D-032 Who controls access: Entra for roles, the app for project visibility

- **Date:** 2026-11-08
- **Decision:** Entra group claims decide only the global role (Admin vs User). Which projects a user can see is assigned inside the app and stored in Postgres, managed from the Admin area. The group-claim to role mapping stays as is.
- **Why:** Entra groups per project do not scale and need an IT ticket each time; role is a stable, security-relevant fact that should stay in the identity provider.

## D-033 Components: modules and raw resources, one schema; links are resolved by the backend

- **Date:** 2026-11-08
- **Decision:** Two component kinds will be supported: (1) a module call and (2) a raw `azurerm_*` resource, for teams that do not want central modules. Both are defined by the same schema mechanism and rendered by the same form engine. Modules remain the default; raw resources are generated only when enabled for a project. On the canvas an arrow between two components means "this one references that one", including across resource groups. The UI never deals with HCL: the generator emits a direct reference in the same folder and a `data` block (team style) across folders.
- **Why:** This is an explicit, opt-in exception to the "module calls, not raw resource blocks" wording of hard rule 5 (update `CLAUDE.md` when raw resources are implemented). Keeping the arrow meaning in the UI and the HCL choice in the backend keeps hard rule 6 intact.

## D-034 Phase list after Phase 5 (supersedes D-031)

- **Date:** 2026-11-08
- **Decision:** Phase 6 Features + UI v2 (major release, local only), Phase 7 Security improvements, Phase 8 ADO + real resources (ADO client and production infra together), Phase 9 Edit existing. Numbers stay sequential even though Phase 6 is a major release. Phase 6 does not read from or push to any repo other than `samples/` and the local repo.
- **Why:** Product features and UI come first; security work reviews the final feature set; the external writer and production infra are proven together at the end.

## D-035 Projects live in the database; generator never overwrites; schema import is validated

- **Date:** 2026-11-08
- **Decision:** (1) Projects, modules, resource types, user assignments and presentation config (e.g. block colour) are stored only in Postgres. Deleting a project removes only the app record; repo files are untouched. An existing repo path can be imported as a new project and its `.tf` turned into components. (2) The HCL generator preserves existing file content: it appends to existing files with `hclwrite`, rejects duplicate module labels, and never rewrites a file wholesale. Incident: a new resource-group node defaulted to `rg.tf` and replaced the existing file, deleting three modules. (3) Schema import validates strictly (valid HCL, every variable typed, defaults match types, supported `validation` blocks, no duplicate names, version required) and shows the converted JSON Schema and a diff against the previous version before saving.
- **Why:** Rule 8 and the round-trip guarantee (untouched bytes unchanged) were violated by the overwrite; schema errors found at import are cheaper than broken forms later.

## D-036 Existing resources are editable in Phase 6; the generator merges, never replaces

- **Date:** 2026-11-08
- **Decision:** (1) Editing resources that already exist in the repository moves from Phase 9 into Phase 6 (it was "not in v1"). A canvas node with `existing {folder, filename, label}` edits that one module call; a node without it adds a new one. Editing a hand-written block in the repo is still limited to module calls (D-033 raw resources come later in Phase 6). (2) `hcl.Apply` merges the canvas into the repository's files: new blocks are appended after the existing content, an edit replaces only the byte range of its `module` block, and every other byte, comment and line ending stays as it was. It fails, instead of overwriting, on a duplicate module label, a data block that already exists with different arguments, an edit whose target is gone, a file that does not parse, and generated output that would not parse. (3) Preview, review and approve all use `hcl.Apply`, so what the reviewer sees is exactly what lands on the branch. Approve writes new files and updated files (`localrepo.File.Update`, refused if the file in the base no longer matches what the plan was built on). The old "conflict" state is gone; failures are `422` with a reason. (4) An edit keeps the repository's own `source` line; a block is regenerated as a whole, so comments written inside an edited module block are not preserved (comments around it are). Attribute-level edits that keep in-block comments are a later refinement.
- **Why:** The `rg.tf` incident (see D-035) showed the generator replaced whole files; the user also needs to open and change existing resources and review them like any other change. A merge layer on top of the existing string generator keeps the golden output unchanged for new files.

## D-037 Access, admin API and module lifecycle (Phase 6 S3)

- **Date:** 2026-11-08
- **Decision:** (1) **Who sees a project:** admins (Entra `devops` group) see all projects; everyone else sees only projects assigned to them in `project_members`. Every `/projects/{id}/...` route is guarded: a non-member gets `404`, not `403`, so a project's existence is not revealed. `projects.entra_group_id` is no longer read. Users are recorded in `users` on their first request (at most once per 10 minutes) and can only be assigned after that. (2) **Admin-only writes:** create, rename/re-describe/re-path (`PATCH`, the slug never changes), delete, member lists, the user list and all module management need the admin role. Delete drops the project, its drafts, changesets and members but never repository files or the audit log; changesets waiting for review make it a `409` unless `force=true`. Every admin action is written to the audit log. (3) **Module lifecycle:** a module version is `published` or `deprecated`. Importing validates first (`catalog.Validate`: JSON, `x-tf-module`, input types, defaults and enums that fit their type, patterns that compile, ranges, `required` that exist, UI groups and `pick-existing` settings, no `$ref`/`oneOf`-style keywords, depth limit) and reports every problem at once. A published version is immutable: the same bytes are a no-op, different bytes under the same version are a `409`. A new version shows what changed against the latest earlier one and which changes break existing projects (input removed, type changed, enum value dropped, input newly required). Deprecated versions leave the developer catalog but stay readable for drafts and changesets already using them; the file seed refreshes a version's schema but never changes its status. (4) **Component settings** such as colours live in `project_component_config`, keyed by `<folder>/<module label>`. (5) **Dev default:** with auth disabled the dev user is now `devops` in the Makefile, `.env.example` and the Helm values, because creating projects and reviewing need the admin role; the dev seed assigns the dev user to the seeded project so a `developer` role still sees it.
- **Why:** D-032 moved project visibility into the app; without a guard on project routes the list would be cosmetic. Immutable published versions keep generated code reproducible, and showing breaking changes before an import is cheaper than finding them in a review.
- **Compatibility:** additive migrations 008 (users, project_members, `projects.description`), 009 (module `status`, `imported_by`) and 010 (component config), each with a working `Down`. After 008, non-admin users see no projects until an admin assigns them. OpenAPI 0.5.0 (additive; `createProject` now rejects unknown fields).

## D-038 Importing existing module calls (Phase 6 S4)

- **Date:** 2026-11-08
- **Decision:** `GET /projects/{id}/import` reads every `module` block in the project's folders (`hcl.ImportModules`) and returns them as canvas nodes with `existing` set. Literal arguments become values; any argument that is not a literal (var, local, module output, conditional, template, for expression, or a literal that contains one) is kept as its source text in `{"$expr": ...}`, so editing a node never turns a reference into a frozen value. The graph now carries `label` and `filename` on module nodes. The generator can write a list of objects or lists (one item per line); before, such a value came out as Go's `[map[...]]` and was not valid HCL. Import is read-only; nothing is written until a changeset is approved.
- **Why:** An edit regenerates the whole block (D-036), so the node must hold the block's real arguments, references included. A test imports every module of AIC and AIH, applies them unchanged and checks the rebuilt graph equals the original.
- **Compatibility:** additive. Graph golden files gain `label` and `filename` on module nodes (regenerated with `UPDATE_GOLDEN=1`; no other change). In-block comments of an edited module are still not kept (D-036).

## D-039 Canvas v2 left panel and real Admin UI (Phase 6 S5-S6)

- **Date:** 2026-11-08
- **Decision:** (1) The canvas left panel has two tabs: Modules (the catalog, drag to add) and Resources (the module calls already in the repository, by folder, with search; click selects one, where "Edit this resource" turns it into an editable node). An imported node replaces the repository node with the same id on the canvas. (2) Draft nodes get edges for every `module.<name>` reference in their values (also inside `$expr`), to repository resources and other drafts in the same folder; they are derived, not stored. (3) HCL Preview has one entry, the nav item, which toggles the side panel; the top bar button is gone. (4) The Admin area uses the real admin API (D-037): module versions (check, import, deprecate/publish), projects (rename, folder, members, delete with force) and users. It replaces the mock.
- **Why:** One place per action, and no mock data that looks real. Reference edges follow what the generated HCL actually depends on.
- **Not done:** the "Approver as mock" item of the plan is not implemented; review keeps using the real devops role.

## D-040 Raw provider resources (Phase 6 S7); hard rule 5 amended

- **Date:** 2026-11-08
- **Decision:** (1) A canvas node can be `kind: "resource"` with a `resource_type` (e.g. `azurerm_resource_group`): it is written as `resource "<type>" "<name>" { ... }` instead of a module call. It has no catalog schema, so its arguments are free-form JSON: plain values as they are, `{"$expr": "var.x"}` for a reference, and `{"<block>": {"$blocks": [{...}]}}` for nested blocks. The type must look like `provider_name`, the name must be a valid Terraform name, and a raw resource cannot be nested in another node. (2) Hard rule 5 in CLAUDE.md now reads "module calls by default; raw resources only when the developer adds one explicitly". The generator never converts a module call into a resource or the other way round. (3) Raw resources follow the same merge rules as modules (D-036): appended to the end of a file, duplicate `<type>.<name>` in a folder is refused (the same name under another type is fine), and an edit replaces only that block and cannot change the type. (4) Importing existing resources: `ImportModules` also returns `resource` blocks with attributes and plain nested blocks. A resource that uses `dynamic` or labelled nested blocks cannot be written back faithfully, so it is not imported and stays untouched. The canvas does not list repository resources yet; only new raw resources, and imported ones through the API, can be edited. (5) The API keeps `kind` and `resource_type` on the canvas node (additive).
- **Why:** Some Azure resources have no module in the catalog yet; without this the developer would have to hand-write `.tf`. Making it an explicit, visibly different node keeps the module-first style and the review diff readable.
- **Risk:** arguments are not checked against the provider schema, so mistakes show up in the pipeline plan, not in TerraFlow.

## D-041 Raw resources are templates an admin prepares; HCL preview shows what changed

- **Date:** 2026-11-08
- **Decision:** (1) A raw resource is an empty provider block such as `azurerm_resource_group` that an **admin** prepares in the Admin area (name, title, resource type, description, optional default folder, default arguments) and developers place on the canvas from the left panel. It is not a catalog module and has no schema. This replaces the free "type any resource" button of D-040: a developer can no longer pick an arbitrary type, only one an admin offered, and the type is read-only on the node. Templates live in `raw_resource_templates` (migration 011); the admin API is `POST/PUT/DELETE /admin/resources` and everyone reads `GET /catalog/resources`; every change is audited. Deleting a template does not touch nodes already on a canvas: they carry their own type and values. (2) The HCL preview now shows only what changed by default: files without changes are hidden and unchanged blocks collapse to one line naming them; a small board above each file names the components it touches and whether each is new, changed or removed. A checkbox shows everything. The panel is narrower. (3) Left panel spacing between categories was increased.
- **Why:** The admin decides which raw resources exist so developers cannot create arbitrary provider resources; this keeps DevOps in control of what bypasses the module catalog. A review reads the change, not the whole file.
- **Bug fixed in the same change (D-038/D-039 follow-up):** a draft holding an imported node crashed the whole page, because a `{"$expr": ...}` value was drawn as a React child. Values from references are now shown as text and kept as written; the form shows a reference as read-only with a "Use a value" button; a module pinned at a version the catalog does not have (v1.2.0 vs v1) maps to the catalog's version of the same major; an imported node that replaces a repository node takes its place on the canvas; and a render error shows a message instead of a blank page.
- **Compatibility:** additive. Migration 011 has a working `Down`. OpenAPI 0.6.0.

## D-042 Admin and Audit log live on Home; full-page HCL preview (Phase 6 S9)

- **Date:** 2026-11-08
- **Decision:** (1) Admin and Audit log are no longer items in a project's nav rail. They are tabs on the Home page, shown only to admins (devops); the Audit log there is global (`GET /admin/audit`, admin only: publishes, approvals, projects, modules, raw resources) while a project's own log stays at `GET /projects/{id}/audit`. (2) The theme toggle moved from the bottom-right corner to the top-right: in the top bar of a project and in the Home nav. (3) Key/value and list inputs always keep one empty row at the end; typing in it adds the entry and opens the next one, so there is no Add button and the row can no longer overflow the panel. A read-only map (such as Tags on a repository resource) is drawn in one two-column grid so keys and values line up. (4) HCL Preview is a page of its own (nav item or "Review diff"), not a side panel; a small board pinned to the top-right of the code names the components the open file touches.
- **Why:** Admin tasks do not belong inside a project; the corner toggle covered content; the Add button was easy to miss and broke the layout.
- **Compatibility:** additive API (`/admin/audit`, OpenAPI 0.6.0 unchanged in shape for existing paths).

## D-043 One changeset waits for review at a time; local sign-in by picking a user; Home layout (Phase 6 S10)

- **Date:** 2026-11-08
- **Decision:** (1) **Publish lock.** A project can have at most one changeset in `published` (waiting for review). While one waits, publishing another is `409` and the body names the one in the way (`pending: {id, title}`); the publish dialog shows it with a link to Review and disables Publish. The old one is decided first: approving it creates the branch, rejecting it frees the slot. The rule is enforced in SQL (`UPDATE ... WHERE NOT EXISTS`), so two publishes at once cannot both win. (2) **Local sign-in.** With `TF_AUTH_DISABLED`, `TF_DEV_LOGIN` (default on) replaces the single fixed dev user with a sign-in page that lists made-up users: Ada Admin (`devops`), Alice and Bob (`developer`). The choice is a cookie; no cookie is a 401 so the UI shows the page. `TF_DEV_LOGIN=false` keeps the old fixed user for scripts. Users are recorded on first request, so an admin can then assign Alice or Bob to a project and the two views can be compared. This is never active when auth is on. (3) **Home.** No top nav: the project cards come first (six at most, then "View all projects" opens a page with search), and below them an Administration section (admin only) with Admin and Audit log. A developer sees no Create, Delete or Administration. (4) **Missing folder is not an error.** A project whose folder is not there (new, or the workspace was wiped) returns an empty graph instead of `500`. The Helm chart now mounts a PVC at `/workspace` (`persistence.enabled`, default on) so project files and approved branches survive a pod replacement; the Deployment strategy is `Recreate` because the volume is ReadWriteOnce. (5) **Canvas.** A new node without a spot (position 0,0) is placed in a free column right of everything drawn and stacked by real height, so it no longer lands on top of repository resources.
- **Why:** Two changesets published in a row could edit the same files and the second would be reviewed against a repository the first had not changed yet. The sign-in is the cheapest way to try role-based views before Entra is wired. A wiped workspace made the whole canvas fail with an unexplained "Loading graph".
- **Compatibility:** additive API (`/api/dev/*` only when the local sign-in is on; publish can now answer `409`). Existing deployments need no migration; with persistence on, the first upgrade starts with an empty workspace volume.

## D-044 Save makes a version; publish happens from Changesets

- **Date:** 2026-11-09
- **Decision:** (1) The top bar's button is **Save**, not Publish. It snapshots the working draft as a new changeset with status `saved`, titled "Version N" (`POST /projects/{id}/changesets/{csId}/versions`). The draft stays the working copy and keeps auto-saving. (2) A saved version is sent for review from the **Changesets** page (Publish, with the title pre-filled); `publish` now accepts a `draft` or a `saved` changeset, and the one-at-a-time publish lock (D-043) is unchanged. (3) `saved` rows belong to their author like drafts: they are listed only to that user, and review, preview and revise treat them as not yet sent. (4) Opening a repository resource for editing no longer adds it to the draft: it joins the draft on the first real change, so looking at a resource does not count as a draft change. (5) The icon at the top of the nav rail, the Login and Home headers and the favicon use the TerraFlow mark.
- **Why:** The draft is saved continuously, so "Publish" at the top asked for a decision (send for review) with no way to keep a named checkpoint first. Versions give a visible history and make publishing a deliberate step in one place.
- **Compatibility:** additive API (`/versions`; `saved` added to the status enum). Migration `012` adds the enum value; its down removes `saved` rows but Postgres cannot drop the value itself.

## D-045 Documentation synced to the Phase 6 build; superseded decisions listed

- **Date:** 2026-11-09
- **Decision:** (1) Phase 6 is marked done in `docs/03-phases.md`; the "Approver as a mock" item is dropped (D-039). (2) Docs were brought in line with the code: the stack as built (no `sqlc`, no OpenAPI code generation, no Monaco, no `react-jsonschema-form`, Tailwind + Harbor/Aurora instead of Fluent/shadcn, `alpine` image with `git`), roles and project visibility (D-032, D-037), Save/Publish (D-044), and the Aurora palette (D-028). (3) Status is kept in `03-phases.md` only; `03-phases.md` also holds the task breakdown (the former `08-dev-plan.md` was merged into it, `CONTRIBUTING.md` into `CLAUDE.md`, and the stale `web/design-preview.html` and `web/phase4.5-mockup.html` were deleted); the README links to the status instead of repeating it. (4) A path with a backslash or a drive prefix (`C:\x.tf`) is now refused by `localrepo` on every OS (it was accepted on Linux, where a backslash is a legal filename byte). (5) Entries below are **superseded or overtaken** and are kept as history: D-012 (edit existing, "Phase 7") by D-036; D-014 and D-016 phase numbers ("Phase 8" is still right for ADO) by D-034; D-029 "editing existing files stays out of scope" by D-036; D-031 by D-034; the Sapphire & Gold palette in `docs/05-design-system.md` by D-028; the "no client secret" line of D-008 is a Phase 8 target, the local build still reads `TF_ENTRA_CLIENT_SECRET`.
- **Why:** The docs still described a design-stage project (Phase 6 "Next", open questions already decided), which makes agents and reviewers act on wrong facts.
- **Compatibility:** docs and one stricter input check; no API, schema or migration change.

## D-046 Phase 7 security tests; changeset scope, string escaping and module names fixed

- **Date:** 2026-11-09
- **Decision:** (1) New security tests (see `docs/03-phases.md`, Phase 7) walk the real router with `chi.Walk`, so a route added without a guard fails a test without anyone updating a list. Four known gaps (PKCE, nonce check, CORS, security headers) are kept as skipped tests that state the wanted behavior. (2) **Fixed, found by those tests:** (a) every route with a `{csId}` looked the changeset up by id only, so a member of one project could read, save, publish or delete another project's changeset; they now pass `ChangesetInProject` first and get `404` unless it belongs to `{id}` (admins too). (b) A malformed project or changeset id, a draft for a module that does not exist, and a project name with a control character answered `500`; they now answer `404` or `400`. (c) The generator wrote string values with Go's `%q`, so a typed dollar-brace interpolation such as a `file()` call, or a percent-brace directive, became live Terraform; string literals now double the dollar or percent sign before an opening brace and write control characters as `\uXXXX`. A reference is written only through `{"$expr": ...}`. (d) A module call's name was not validated (raw resources were); it must now match `^[A-Za-z_][A-Za-z0-9_-]*$`. (3) The dev sign-in is kept.
- **Why:** The hard rule against interpolating unvalidated input into HCL, and project routes must not leak or allow cross-project access (D-037).
- **Compatibility:** generated HCL is unchanged for every sample and golden file; only typed text that contains a dollar-brace or percent-brace sequence changes. A developer who typed a reference into a text field must use the reference option (`$expr`) instead. No API shape, schema or migration change; some requests that returned `500` now return `404` or `400`.
- **Not changed (needs a decision):** inside a project any member can still open another member's draft by id (D-044 says drafts belong to their author, the API does not enforce it).

## D-047 Phase 7 login and API hardening

- **Date:** 2026-11-09
- **Decision:** (1) Login uses PKCE (S256): the verifier is a short-lived httpOnly cookie and the code exchange proves it; the ID token's nonce must equal the `tf_oidc_nonce` cookie or the callback is refused. (2) The `sessions` table stores the SHA-256 of the session id, so a leaked backup or SQL read is not a usable cookie; creating a session also deletes expired rows. Existing sessions stop working once, at deploy. (3) CORS no longer answers `*`: only origins in `TF_ALLOWED_ORIGINS` (comma separated, empty by default) are allowed, with credentials. The app is same-origin, and the Vite dev proxy keeps the browser's Host, so local development needs no setting. (4) Every response carries `X-Content-Type-Options`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Cache-Control: no-store` and a CSP that forbids everything (the API only returns JSON); HSTS is added unless `TF_INSECURE_COOKIES` is set. (5) POST/PUT/PATCH/DELETE are refused with 403 when the browser marks them `cross-site` or `same-site`, or sends an `Origin` that is not this host (or `X-Forwarded-Host`) or an allowed origin. Requests with neither header (scripts) pass and still need a session. (6) `/auth/login` and `/auth/callback` allow 20 requests at once per client address, then one per 6 seconds, in memory per pod. (7) The server exits at start when `TF_AUTH_DISABLED=true` and any `TF_ENTRA_*` setting are both present. The dev sign-in itself is unchanged.
- **Why:** Closes the open items of D-046: PKCE and nonce are standard for an authorization-code flow, a wildcard CORS answer contradicted the same-origin design (D-011), and a copied production configuration must not silently turn sign-in off.
- **Compatibility:** all users sign in again once after deploy (session hashing). The Helm chart and Makefile set no Entra variables with `TF_AUTH_DISABLED`, so they still start. Behind an ingress that rewrites `Host`, forward the original in `X-Forwarded-Host` or list the public origin in `TF_ALLOWED_ORIGINS`, or writes get 403. No API shape or migration change.

## D-048 Phase 7 close-out: private drafts, 4-hour sessions, validated env, accessibility, non-root image

- **Date:** 2026-11-09
- **Decision:** (1) **Drafts are private (D-044 enforced):** a draft or saved version answers `404` to everyone but its author, admins included; once published, members and admins read it. (2) **Sessions last 4 hours** (was 8). Roles are read once at sign-in and not refreshed (no Graph calls, D-010), so the session length is the bound on a stale role. (3) **`env` is validated** (`^[A-Za-z0-9][A-Za-z0-9_-]{0,31}$`) in the graph reader, the graph and changeset endpoints and the branch name; it became part of `env/<env>.tfvars`, so `../x` could read a `.tfvars` file outside the project. Graph and project errors are now written as JSON with proper escaping. (4) **Accessibility:** colour tokens for muted text, success and info badges were darkened in the light theme (and kept in the dark theme) to reach 4.5:1; every page has a `main` landmark and an `h1`; the catalog panel shows its own error instead of crashing the page when `/api/catalog` fails; the changesets page no longer says "No changesets yet" next to an error. (5) **The Vite dev proxy keeps the browser's Host** (`changeOrigin: false`), because the same-origin check of D-047 compares `Origin` with `Host`; Vite's default rewrote it and made every dev POST fail. (6) **Browser tests** (`web/e2e`, Playwright with axe-core) cover accessibility in both themes and the error, empty and loading states; they run against a stack that is already up. (7) **Dependencies:** vite 6.4.6 and react-syntax-highlighter 16.1.2 close the `npm audit --omit=dev` findings. (8) **The image runs as UID 65532** and the chart sets `runAsNonRoot`, drops all capabilities and sets `fsGroup`. (9) Threat model, secret checklist and identity-scope audit are in `docs/10-security.md`; the manual is `docs/11-user-manual.md`.
- **Why:** Closes Phase 7 for the local build. The `env` traversal and the dev-proxy regression were found by running the application, not by reading it.
- **Compatibility:** none to the API shape. Existing sessions are not affected by (2) until they expire; someone who saved drafts and relied on a colleague opening them by id no longer can. **Not verified:** the container and chart changes were not run on a cluster (no Docker daemon or Kubernetes in the build environment), and `govulncheck` could not run (network policy).

## D-049 Phase 8 is a major release: GitHub, user-owned projects; the Azure service plan is dropped; hard rule 3 amended

- **Date:** 2026-11-09
- **Decision:** (1) **Git provider.** GitHub is the git provider. Azure Repos is a future feature behind the same interface; GitLab is not planned. (2) **No Azure services.** The plans for AKS, Azure PostgreSQL with Entra, Key Vault, Workload Identity and Azure DevOps as a service are dropped. `azurerm` stays the main Terraform provider; TerraFlow still never calls Azure (rule 2). (3) **Sign-in.** An Entra app registration (free) stays, and sign-in with GitHub is added (D-050). (4) **Hosting.** Not decided; Vercel and Supabase are no longer part of the plan, and go live is a later item. The code keeps to three principles so any host works: no new disk writes, no `git` binary, no in-process state; one database URL; migrations as a separate command. (5) **Hard rule 3 amended** (see CLAUDE.md): no Azure credential; the git credential is only what can create a branch on the chosen repository, never returned to the UI or logged; secrets only in the environment; no pasted personal tokens. (6) **D-013 is withdrawn:** a GitHub Actions workflow runs the Go and web checks with a Postgres service. (7) `localrepo` stays as the `local` provider for development and tests. (8) The sample modules for tests come from `github.com/Anmewzaa/infrastructure-terraform-modules`.
- **Why:** The product is for people with GitHub repositories and Terraform for Azure. An Azure hosting and identity plan needs resources that are not available, and it was making the plan harder to read.
- **Supersedes:** D-014 and D-034 (Phase 8 contents), D-003 and D-009 (Azure PostgreSQL, one Workload Identity), D-013, and the hard rule 3 wording. D-008 (federated credential) is not pursued.
- **Compatibility:** none yet; the schema changes are in D-050.

## D-050 User-owned projects, roles, a two-layer GitHub connection, two sign-in providers, scoped catalogs

- **Date:** 2026-11-09
- **Decision:** (1) **Two-layer GitHub model.** Layer 1: whoever owns a GitHub account or organisation installs the GitHub App on it; TerraFlow stores only the installation id and account; one-hour installation tokens are minted per call from a JWT signed with the App's private key, limited to one repository, `contents: write` and `metadata: read`, never stored. Layer 2: a user connects a repository to a project by authorizing with GitHub; TerraFlow lists the repositories that the installation can see and that this user can access, and the user picks one. The user token is used during the request and discarded. (2) **Projects belong to users.** Any signed-in user creates a project from a repository they can access and becomes its owner. Roles: owner (all, including approve, invite, set roles, disconnect, delete, transfer), co-owner (approve, invite, set roles below owner, manage the project's schemas; no delete or transfer), contributor (build, save, publish; no approve). The owner may approve their own changes; a per-project setting can require a second reviewer. This replaces the admin-only assignment of D-032 and D-037 for new projects; the global `admin` role remains. (3) **Admin visibility.** The admin sees every project's metadata and may disconnect, transfer or force a re-check. Opening a project's canvas or diff needs "support access", audited each time. This is a default and can be loosened. (4) **Allowlist.** `TF_GITHUB_ALLOWED_OWNERS` limits which accounts may install; empty means any. (5) **Identity providers.** `github` (OAuth2 through the same App, no ID token) and `entra` (OIDC). Users are `github:<id>` and `entra:<oid>`; unprefixed ids are Entra. No automatic account linking. `TF_ADMIN_USERS` names global admins. (6) **Catalog scopes.** A global catalog (admin) and project catalogs (owner and co-owner); contributors use, not edit. Raw resource templates follow the same scopes. (7) **Provider-neutral schemas.** A schema declares its Terraform provider, domain and icon; code never branches on module or provider names; `azurerm` is the first provider, not a hardcoded one. (8) **Connection data is provider-neutral** (provider, external account, a provider-specific reference without secrets) so an Azure Repos connector can reuse it. (9) **Sign-up approval.** The default `TF_SIGNUP_MODE=approval` makes a new user `pending` until an admin approves; the other modes are `open` and `invite_only`. User states: `pending`, `active`, `rejected`, `suspended`. A pending user can do nothing but see a waiting page; an invitation by an owner does not bypass approval; `TF_ADMIN_USERS` are active at once; optional auto-approval rules (`TF_AUTO_APPROVE`) for an Entra tenant, GitHub organisations or verified email domains. Users are identified by id, never by login name; the pending count is capped; reject, approve and suspend are audited; suspending ends the user's sessions at once; a suspended owner's project stops publishing and approving until an admin transfers it. The admin does **not** approve projects or changesets: that stays with the project owner and co-owner.
- **Why:** People should be able to use TerraFlow with their own repositories. Checking the user's own access at connect time stops the App from being a way into repositories the user cannot reach. A flexible catalog is needed once many users bring their own modules.
- **Compatibility:** additive migration 013 (connections, project repository columns, member roles); existing members become contributors; existing projects keep working with the `local` provider. OpenAPI additions for connections, the repository picker, roles and invitations.
- **Risks:** one App private key can mint tokens for every installation (see `docs/10-security.md`, G1); anyone may try to sign in, so approval is the default and per-user limits are still needed for the `open` mode; a contributor acts through the owner's connection, which the owner can lose access to later.
- **Open:** whether admin should see canvases freely; per-user limits for the `open` mode; the auto-approval rules to ship first; the azurerm resource list for M3.

## D-051 Phase 8 foundations: one provider interface, snapshots as file systems, branch from the planned commit

- **Date:** 2026-11-09
- **Decision:** (1) **One interface.** A project's repository is read and written only through `gitprovider.Source`: `Snapshot` (the project folder at one commit, as an `fs.FS`) and `CreateBranch` (one commit on a new branch). Handlers receive a `Repos` function and never open a folder themselves. `Local` keeps the folder and `git` binary for development and tests; `github` uses the API; `Memory` has neither disk nor binary. Listing a user's repositories is part of the connect flow, not of `Source`. (2) **`hcl` reads from `fs.FS`** (`BuildGraphFS`, `ImportModulesFS`, `ParseFolderFS`); the directory functions are wrappers, so no golden file changed, and a snapshot held in memory gives the same graph as the folder (tested on both sample projects). (3) **The branch is made from the commit the plan was built on** (`Snapshot.Commit` is passed back as `BranchRequest.Base`), so the branch holds exactly what the reviewer saw. This replaces the wording in the plan that said Approve would be refused with 409 when the base moved; if the default branch moved, GitHub shows the difference in the pull request. The checks that a new file does not exist and that an updated file still has the content the plan started from remain (D-036). The `local` source branches from the checked-out HEAD, as before. (4) **A repository that cannot be read stops the plan.** Before, an unreadable folder meant an empty list of files; with a remote provider that would regenerate everything. Provider errors map to statuses: not found 404, credential refused 409 ("needs attention"), rate limited 429, name or file conflicts 409, unsafe path 422. (5) **GitHub client.** A JWT of nine minutes that starts a minute in the past is exchanged for an installation token limited to one repository with `contents: write` and `metadata: read`; a token is made per operation and never stored or reused; a 401 mints a new one once; a 422 when making a token means the installation no longer includes the repository and is reported as a refused credential. Only `.tf` and `.tfvars` files are downloaded, files over 256 KB are skipped, more than 500 files or a truncated tree is an error (`ErrTooLarge`), never a partial read. Responses are limited to 16 MB; error messages carry the status and GitHub's short message, never a header; `App` and `Token` print as hidden. (6) **A contract suite** (`gitprovidertest`) is run by every source, and a fake GitHub (`githubtest`) enforces the real protocol (signature, issuer and lifetime of the JWT; a known, unexpired token limited to its repository and permissions), so the client is tested against the rules, not against itself. (7) **Data model (migration 013).** `connections` (provider, external id, account, status, creator; no secret), `project_repos` (connection, repository id and name, default branch, folder, who connected it, the second-reviewer switch) as a separate table so a local project has no empty columns, and `project_members.role` with an index that allows one owner per project. Store functions cannot create a second owner: `AddMemberRole` refuses `owner`, and ownership changes only through `CreateOwner` and `TransferOwnership`. (8) **Known gaps, deliberately left:** the generator still writes a placeholder module `source` (`buildModuleSource`); reads are not cached by commit SHA; the connect flow, the repository picker, the roles in the API and the screens are not built.
- **Why:** The same flow must run on a folder, in memory and on GitHub, and must not depend on the machine. Branching from the planned commit makes the review exact without a race to detect. Failing on an unreadable repository prevents duplicated resources.
- **Compatibility:** none for local projects (all earlier API, golden and round-trip tests pass unchanged). Additive migration 013 with a working `Down`. Handler constructors take a `Repos` argument (internal). No OpenAPI change yet.
- **Open:** caching by commit SHA; how the connect flow stores the GitHub account that authorized; the `source` of generated modules.

## D-052 Phase 8 M1 completed: grants for layer 2, project roles in the API, admin support access
- **Date:** 2026-11-09
- **Decision:** (1) **Layer 2 without storing a token.** After the user authorizes, the callback uses their token once to list the installations and the repositories they can reach, saves one short-lived *grant* per repository (migration 014, `repo_grants`: user, connection, repository, default branch, the GitHub account that authorized, expiry ten minutes) and forgets the token. The pickers read grants; `POST /projects/from-repo` needs a live grant and uses it up. This answers the open question of D-051 on where the authorizing GitHub account is kept: on the grant and in the audit entry, not on the connection. (2) **Roles in the API.** Approve and reject accept the global `devops` role or the project's owner and co-owners; the per-project second-reviewer switch (on `project_repos`) forbids approving your own change even for the owner. Owner and co-owner invite, set roles (never to or from owner), remove; only the owner transfers or deletes (a GitHub project; an admin still deletes any project). The admin's replace-all member list no longer deletes the owner. (3) **Admin support access.** An admin opening a project they are not a member of is allowed and audited as `support.access` on every request. (4) **GitHub not configured is a 409,** not a 503: the route-matrix test requires that no request is answered with a 5xx, and this is a setting the operator has not made. (5) **Return to the screens.** The screens route by URL hash, so the callback redirects to `/#/projects/new?connected=N` or `?error=code`, prefixed by `TF_WEB_URL` when the screens are served elsewhere. (6) `terraflow migrate` runs migrations and the catalog seed and exits; `TF_MIGRATE_ON_START=false` skips them at start. (7) A GitHub Actions workflow runs the Go and web checks.
- **Why:** A grant gives "this user could reach this repository a moment ago" without keeping a credential, works on any host (it is in the database, not in memory) and makes the confused-deputy case (G4) impossible to reach by guessing ids. Roles in the API, not only in the screens, are what stop a contributor from approving.
- **Compatibility:** additive migration 014 with a working `Down`; new endpoints only (see `api/openapi.yaml`); the review rule widens who may approve and adds the optional second-reviewer rule; an admin's member replacement keeps the owner.
- **Open:** a run against real GitHub with the owner's sandbox repository; reads cached by commit SHA; the database rate limiter; `source` lines in the catalog; Playwright tests for the new screens; grants for more than 500 repositories are cut off.

## D-053 Phase 8 M1 leftovers and M2: SHA cache, shared rate limiter, module sources, sign-in with GitHub, sign-up status
- **Date:** 2026-11-10
- **Decision:** (1) **Read cache by SHA.** The GitHub source keeps a bounded in-memory copy of snapshots (by tree SHA) and blobs (by blob SHA). This is the one exception to "no in-process state" in D-049: the data is immutable, so it can never be stale, and losing it only costs requests. (2) **Rate limiter in the database.** The sign-in limiter takes tokens from a row per client address (migration 015, one transaction with a row lock) so every instance shares one count; if the database fails it falls back to the in-memory limiter. (3) **Module source.** A catalog schema may carry a full `x-tf-module.source`; a bare name goes under `TF_MODULE_SOURCE_BASE`; `?ref=<version>` is added unless present. The catalog is re-read at most once a minute. The `//modules/<name>` layout of `infrastructure-terraform-modules` is an assumption. (4) **Sign-in with GitHub** through the same App: `GET /auth/github/login` and `/callback`, a random `state` cookie, the user token used once to read the numeric account id and dropped. No PKCE. The user id is `github:<id>`; Entra ids keep their unprefixed form, so nothing is migrated and the two are never linked. (5) **Sign-up status** (migration 016): `pending`, `active`, `rejected`, `suspended`, checked on every request; a pending user may call only `/api/me`; rejecting or suspending deletes sessions; a pending session never carries the admin role. `TF_SIGNUP_MODE` is `approval` (default), `open` or `invite_only`; `TF_ADMIN_USERS` are active at once; the pending list is capped (`TF_SIGNUP_MAX_PENDING`). (6) **Admin approval** through `POST /admin/users/{oid}/status`, audited, with a reason for reject and suspend, never on one's own account; pending or inactive users cannot be invited or assigned. (7) **Login page** draws a button for each provider from `GET /api/auth/providers`.
- **Why:** The cache and the shared limiter close the M1 items left open by D-051 and D-052. GitHub sign-in with approval by default lets anyone try TerraFlow without letting anyone use it, which is the privacy and abuse stance of D-050.
- **Compatibility:** additive migrations 015 and 016 (existing users stay `active`, provider `entra`); `openapi.yaml` 0.7.0 adds two endpoints and three user fields; a session whose user has no row in `users` is refused once, so that person signs in again. With `TF_LOGIN_PROVIDERS` unset the behavior is Entra only, as before.
- **Open:** `TF_AUTO_APPROVE` rules; limits per user for `open`; re-checking an owner's repository access at sign-in; stopping a suspended owner's project; the first run against real GitHub and Entra; BUG-1.
