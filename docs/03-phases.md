# 03 — Phases

Status: agreed (v0, 2026-11-07)

Principles:

- Every phase ends in something that can be demonstrated.
- Read-only work comes before any write work.

| Phase | Work | Exit criteria |
|---|---|---|
| **1. Visualizer (local)** | Go parser: `.tf` folders → graph JSON (module calls, `data` blocks, cross-folder references, evaluation of `var`/`locals`/`.tfvars`). Rendered with React Flow + ELK. No auth, no DB, no ADO | `samples/iac/project-1` renders as the expected graph in `samples/README.md`, including cross-folder references and hub resources shown as external |
| **2. Foundation (local)** | Entra BFF login, Project scoping, Postgres (local), chi router, Docker + Helm for local k8s. Local samples only, no ADO | A developer logs in via Entra, sees their projects, views the graph. App is still read-only. Runs on local k8s |
| **3. Module catalog** | Decide how modules are added (at phase start). `variables.tf` → JSON Schema → form. Output layout config | A new module can be added with no UI code changes |
| **4. Canvas editing + drafts** | Agree the UI design first. Drag and drop modules, link to existing resources (locked derived values), HCL generator, Save/Publish | DevOps signs off that generated `.tf` matches what the team would write by hand |
| **5. Review + branch (local)** | Review screen (canvas diff + HCL diff), Approve → create branch on local test repo, Reject with a reason → back to the developer, audit log | Full flow works end to end against a local test repo |
| **6. Hardening** | Security review, DevOps user manual, open issues | Ready for production use |
| 7. Edit existing (after go-live) | Modify existing and hand-written resources | Not planned yet |
| **8. Production infra** | Workload Identity (managed identity), Azure PostgreSQL (Entra passwordless), Key Vault, read repos from ADO, deploy to AKS, federated credential (no client secret) | App runs on AKS with real ADO repos, no local secrets |

**v1 = Phases 1–6.** Phase 8 upgrades from local to production infrastructure.

## Parallel work during Phase 1

Request access early, because approvals in a bank take time:

- Entra app registration, federated credential and group assignment
- Workload Identity (user-assigned managed identity) and its permissions in the ADO org
- Azure PostgreSQL Flexible Server (Entra auth) and Key Vault
- A test ADO repo for Phase 5

## Before Phase 1

- DevOps reviews the conventions table in `samples/README.md` and corrects the samples where they differ from the real repos.
