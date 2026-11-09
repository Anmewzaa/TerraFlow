# 01 — Scope

Status: agreed (v0, 2026-11-07; updated 2026-11-09 for D-032, D-036, D-037, D-043, D-044)

## In-app flow

The app's responsibility ends when the branch is created.

```mermaid
flowchart LR
    A[Developer: build on canvas] --> B[Save draft]
    B --> A
    A --> C[Publish for review]
    C --> D{DevOps reviewer}
    D -- Reject + reason --> A
    D -- Approve --> E[Generate .tf]
    E --> F[Create branch on the git repo]
    F -.-> G[PR / pipeline / plan / apply<br/>done by DevOps in the git provider]

    style G stroke-dasharray: 5 5
```

1. A developer opens a **Project** and builds on the canvas by dragging and dropping modules.
2. The working copy is auto-saved as a draft in Postgres. **Save** snapshots it as a named version ("Version N"); **Publish**, from the Changesets page, sends a saved version for review (D-044). A project has at most one changeset waiting for review at a time (D-043).
3. A **DevOps reviewer** reviews it in the web UI with a canvas diff and an HCL diff.
   - There is **no `terraform plan` in the app**. The real plan is reviewed in the PR pipeline of the git provider.
4. **Approve:** Terraflow generates `.tf` (module calls and `data` blocks) and creates a new branch on the project's git repo (GitHub first; GitLab and Azure DevOps later, D-049).
5. **Reject:** the reviewer must give a reason. The request goes back to the developer, who can edit and publish again.

## Two-stage approval

| Stage | Where | Purpose |
|---|---|---|
| 1 | Terraflow | Approve before any branch exists, so unreviewed requests cannot create branches |
| 2 | PR review in the git provider | Normal DevOps process. Terraflow has no part in it |

## v1 boundaries

- **In:** create new resources (as module calls, or as raw resources an admin prepared, D-040/D-041), edit module calls that already exist in the repo (D-036), and reference existing resources.
- **Editing existing resources:** module calls already in the repository can be edited from Phase 6 (D-036). **Not yet:** hand-written blocks that cannot be written back faithfully (`dynamic` or labelled nested blocks), and edits that keep comments inside the edited block (Phase 9).

## Out of scope

- PR creation, merge, pipelines, plan, apply
- Any call to Azure (read or write)
- Notifications (Teams, email)
- A CI pipeline for Terraflow itself

## Users and roles

- The global role comes from **Entra ID group claims** at SSO login (D-032). There is no role screen.
  - Platform/DevOps group → **DevOps / Admin** (review, approve, manage modules, raw resources, projects and users; sees every project)
  - App-team group → **Developer** (build on canvas, request from approved modules)
- A **Project** maps to one git repository and one folder in it, both picked from lists, never typed (D-049); a folder in the local repo for development. **Which projects a user sees is assigned inside the app** (`project_members`, managed by an admin) and stored in Postgres, not by Entra groups (D-032, D-037). A non-member gets `404` on a project.
- With Entra disabled (local build only) a sign-in page lists made-up users so both roles can be tried (D-043).

## Data ownership

| Data | Stored in |
|---|---|
| Projects, members, users, module catalog and raw resource templates, drafts, saved versions, review status, reject reasons, audit trail (who approved what, and when) | PostgreSQL (Terraflow's own DB, which may also serve other uses later) |
| Infrastructure definition | The project's git repository (single source of truth) |
| Terraform state | Not touched by Terraflow |

## Referencing existing resources (cross-folder)

The team's convention is that one folder refers to resources from another folder
with `data` sources (see `samples/README.md`).

- Terraflow builds an **inventory** of existing resources by parsing `.tf` on the
  repo's main branch. It uses the same engine as the read-only visualizer.
  Terraflow does **not** read from Azure.
- A developer can pick only resources from their own project's repo. Values that
  can be derived (for example, the resource group of a chosen VNet) are filled in
  and **locked**.
- Terraflow generates the required `data` blocks in the team's style, so DevOps
  does not hand-write them.
- **Known constraint:** resource names built from `var`, `locals` and `.tfvars`
  must be evaluated per environment.
- **Future option (not in scope):** check that a resource really exists, using
  Azure Resource Graph with the Reader role.
