# 03 — Phases

Status: agreed (v1, re-planned 2026-11-08; see D-034. Updated 2026-11-09: Phase 6 done)

Principles:

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
| **7. Security improvements** | Planned | Security review (threat model, secret handling, identity scope), RBAC edge cases, error/empty/loading states, perf on 80+ node canvases, accessibility, DevOps user manual, open issues | Ready for production use |
| **8. ADO + real resources** | Planned | `internal/ado` (read repo `.tf` from `main`, create branch via Pushes API) behind the same interfaces as `localrepo`; Workload Identity, Azure PostgreSQL (Entra passwordless), Key Vault, federated credential, deploy to AKS | Publish, approve, then a branch appears in a real ADO repo; app runs on AKS with no local secrets |
| 9. Edit existing, advanced (after go-live) | Not planned | Attribute-level edits that keep comments inside blocks, hand-written resources that are not module calls | Not planned yet |

**Versioning:** Phase 6 ships as a major version (v2) but phase numbers continue sequentially. Phases 1-8 are the path to production. Phase 6 neither reads from nor pushes to any repo other than `samples/` and the local repo.

**Dropped from Phase 6:** the "Approver page kept as a mock UI" item (6.2.5). Review uses the real DevOps role and the real Phase 5 backend (D-039). Task-level detail for every phase is in [08-dev-plan.md](08-dev-plan.md); this page only tracks status.

**Carried into Phase 7/8:** DevOps sign-off of the generated `.tf` (Phase 4 exit); the Entra app still uses a client secret locally, and the federated credential arrives with Phase 8; `internal/ado` is an empty package until Phase 8.

## Parallel work during Phase 1

Request access early, because approvals in a bank take time:

- Entra app registration, federated credential and group assignment
- Workload Identity (user-assigned managed identity) and its permissions in the ADO org
- Azure PostgreSQL Flexible Server (Entra auth) and Key Vault
- A test ADO repo for Phase 8 (request now; needed for Phase 8)

## Before Phase 1

- DevOps reviews the conventions table in `samples/README.md` and corrects the samples where they differ from the real repos.
