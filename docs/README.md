# Terraflow documentation

Terraflow is a visual, drag-and-drop portal for Azure infrastructure. Developers
compose resources on a canvas from modules that the DevOps team has approved.
A DevOps reviewer approves the request, and Terraflow then generates Terraform
and creates a branch on the project's Azure DevOps (ADO) repo. From that point
the DevOps team takes over in ADO (PR, pipeline, plan, apply).

Terraflow never calls Azure and holds no Terraform state. The Git repo is the
single source of truth for infrastructure.

## Documents

| Doc | Contents |
|---|---|
| [01-scope.md](01-scope.md) | What the app does, what it does not do, roles, data |
| [02-tech-stack.md](02-tech-stack.md) | Architecture and technology choices |
| [03-phases.md](03-phases.md) | Phase status, exit criteria and the task breakdown per phase (BE/FE/infra/tests) |
| [04-decisions.md](04-decisions.md) | Decision log with the reasoning behind each choice |
| [05-design-system.md](05-design-system.md) | The "Harbor" design system: identity, tokens, interaction contract |
| [06-conventions.md](06-conventions.md) | Code conventions (Go, HCL generation, API, React, tests) |
| [07-glossary.md](07-glossary.md) | The project's own vocabulary |
| [09-graph-contract.md](09-graph-contract.md) | Canvas graph JSON shape + HCL resolution rules (BE↔FE contract) |
| [10-security.md](10-security.md) | Threat model, secret-handling checklist, identity-scope audit, accepted risks |
| [11-user-manual.md](11-user-manual.md) | Manual for developers and DevOps (admins) |
| [../CLAUDE.md](../CLAUDE.md) | Contributor & AI agent guide (hard rules, ownership, verification) |
| [../samples/README.md](../samples/README.md) | Synthetic IaC samples and the conventions they assume |

## Where things stand

Phases 1-7 are done against `samples/` and a local repo. Phase 8 (GitHub, user-owned
projects, sign-in with GitHub and Entra, D-049 and D-050) is the current work. Status lives in [03-phases.md](03-phases.md)
only; do not repeat status elsewhere, link to it.

## Open items

| Item | When |
|---|---|
| Owner creates the GitHub App (any account, callback on `localhost`) and a sandbox repository (steps in the plan, 8.1) | Before the live GitHub test in M1 |
| Decide whether the admin may open users' canvases freely (default: audited support access) | Before M1 ends |
| Per-user limits for the `open` sign-up mode; which auto-approval rules to ship first | Before go live |
| Agree the list of `azurerm` resources and the schema editor design | Before M3 |
| Generated module `source` comes from the catalog (`x-tf-module.source`) or `TF_MODULE_SOURCE_BASE`; confirm the layout of `infrastructure-terraform-modules` | Phase 8, M1 |
| Cache repository reads by commit SHA; `terraflow migrate` as a separate command; rate limiter in the database | Phase 8, M1 |
| Hosting and go live (host, domain and redirect URIs, database URL, environment) | Not decided |
| Azure Repos provider | Future |
| DevOps sign-off that the generated `.tf` matches what the team writes by hand (Phase 4 exit) and of the 9 conventions in `samples/README.md` | Before a team relies on it |
| `govulncheck`, container settings on a cluster, browser frame rate with 80+ nodes | Where the tooling is available (see [10-security.md](10-security.md)) |
| OpenAPI code generation and drift check; `golangci-lint` config | After Phase 8 |

Closed: module registration (D-015), UI component library and palette (D-026,
D-028), the security review and user manual (Phase 7).
