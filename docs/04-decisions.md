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
