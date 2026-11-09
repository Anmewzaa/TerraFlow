# 07 — Glossary

The project's own words, so everyone (and any AI agent) reads a request the same
way.

| Term | Meaning in TerraFlow |
|---|---|
| **Project** | One unit of work = one git repository and one folder in it, picked from lists (a folder in the local repo for development). Stored in Postgres. A developer sees only the projects an admin assigned to them (D-032). |
| **Admin** | A user in the Entra DevOps group (role `devops`). Sees every project; manages modules, raw resources, projects and users; reviews and approves. |
| **Changeset** | The unit of work: all nodes, values and edges one developer adds or edits in a project. It is a draft, then a saved version, then published, then approved or rejected. |
| **Existing node** | A canvas node imported from a module call already in the repo (`existing {folder, filename, label}`). Editing it rewrites only that block (D-036). |
| **Raw resource** | A provider `resource` block (for example `azurerm_resource_group`) added from an admin-prepared template instead of a catalog module (D-040, D-041). |
| **Canvas** | The drag-and-drop surface (React Flow) where a developer composes resources. |
| **Module** | A DevOps-authored Terraform module in the central module repo. Its schema drives the form; TerraFlow emits a call to it. |
| **Module catalog** | The global set of registered module versions with their schemas and UI metadata (D-017). A version is `published` or `deprecated` (D-037). |
| **Schema** | A module's input definition (`variables.tf` → JSON Schema) that renders the form. "Schema-driven" = forms come from this, never hardcoded. |
| **Layout config** | Repo config that says which folder/root-module each generated file goes into (matches the team's structure). |
| **Inventory** | The list of resources that already exist, built by **parsing `.tf` in the repo** (never from Azure). Used to reference existing resources. |
| **Root module / folder** | One folder in the IaC repo with its own state (e.g. `common/`, `vnet/`). Cross-folder references use `data` sources. |
| **Draft** | The working copy of a changeset, auto-saved in Postgres, not yet submitted. |
| **Version (Save)** | A snapshot of the draft with status `saved`, titled "Version N" (D-044). |
| **Publish** | Send a saved version for DevOps review from the Changesets page. Only one changeset per project can wait for review (D-043). |
| **Review** | A DevOps reviewer reads the canvas diff + HCL diff in the web UI. No `terraform plan` happens in the app. |
| **Approve** | The reviewer accepts; TerraFlow generates `.tf` and creates a **branch** on the project's git repository. The app's job ends here. |
| **Reject** | The reviewer sends it back with a required reason; the developer edits and publishes again. |
| **Branch** | The git branch TerraFlow creates on Approve. DevOps takes it through PR/pipeline/merge in the git provider. |
| **Locked / derived value** | A field TerraFlow fills and freezes (e.g. the RG of a chosen VNet) so it can't be set wrong. |
| **unknown** | An inventory value that couldn't be evaluated from `var`/`locals`/`.tfvars`. Shown as `unknown`, never `0`/blank. |
| **Resource domain** | The Azure grouping used for node color/icon: network, compute, data, security (+ neutral for resource groups, integration, external). |
| **External / hub reference** | A `data` source or hub resource the repo reads but does not create. Shown as external, styled neutral. |
| **BFF** | Backend-for-frontend: the Go server runs the Entra OIDC flow and keeps a server-side session; tokens never reach the browser. |
| **Workload Identity** | An Azure managed identity. Planned for the Azure profile of Phase 8, dropped by D-049; the demo profile does not use it. |
| **Git provider** | GitHub (first), GitLab or Azure DevOps. TerraFlow reads the repository and creates the branch through the provider's API. |
| **GitHub App / installation token** | The way TerraFlow gets access to GitHub: an admin installs the App on chosen repositories; TerraFlow stores only the installation id and, per call, gets a one-hour token limited to one repository. |
