# 07 — Glossary

The project's own words, so everyone (and any AI agent) reads a request the same
way.

| Term | Meaning in TerraFlow |
|---|---|
| **Project** | One unit of work = one ADO repo/project + one Entra group. A developer sees only the projects whose group they belong to. |
| **Canvas** | The drag-and-drop surface (React Flow) where a developer composes resources. |
| **Module** | A DevOps-authored Terraform module in the central module repo. Its schema drives the form; TerraFlow emits a call to it. |
| **Module catalog** | The set of modules a project may use, with their schemas and UI metadata. |
| **Schema** | A module's input definition (`variables.tf` → JSON Schema) that renders the form. "Schema-driven" = forms come from this, never hardcoded. |
| **Layout config** | Repo config that says which folder/root-module each generated file goes into (matches the team's structure). |
| **Inventory** | The list of resources that already exist, built by **parsing `.tf` in the repo** (never from Azure). Used to reference existing resources. |
| **Root module / folder** | One folder in the IaC repo with its own state (e.g. `common/`, `vnet/`). Cross-folder references use `data` sources. |
| **Draft** | Work saved in Postgres but not yet submitted. |
| **Publish** | Submit a draft for DevOps review. |
| **Review** | A DevOps reviewer reads the canvas diff + HCL diff in the web UI. No `terraform plan` happens in the app. |
| **Approve** | The reviewer accepts; TerraFlow generates `.tf` and creates a **branch** on the ADO repo. The app's job ends here. |
| **Reject** | The reviewer sends it back with a required reason; the developer edits and publishes again. |
| **Branch** | The git branch TerraFlow creates on Approve. DevOps takes it through PR/pipeline/merge in ADO. |
| **Locked / derived value** | A field TerraFlow fills and freezes (e.g. the RG of a chosen VNet) so it can't be set wrong. |
| **unknown** | An inventory value that couldn't be evaluated from `var`/`locals`/`.tfvars`. Shown as `unknown`, never `0`/blank. |
| **Resource domain** | The Azure grouping used for node color/icon: network, compute, data, security (+ neutral for resource groups, integration, external). |
| **External / hub reference** | A `data` source or hub resource the repo reads but does not create. Shown as external, styled neutral. |
| **BFF** | Backend-for-frontend: the Go server runs the Entra OIDC flow and keeps a server-side session; tokens never reach the browser. |
| **Workload Identity** | The single Azure managed identity the backend uses (passwordless) for Postgres, Key Vault and ADO, least-privileged per target. |
