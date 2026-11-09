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
| [03-phases.md](03-phases.md) | Delivery phases and exit criteria |
| [04-decisions.md](04-decisions.md) | Decision log with the reasoning behind each choice |
| [05-design-system.md](05-design-system.md) | The "Harbor" design system: identity, tokens, interaction contract |
| [06-conventions.md](06-conventions.md) | Code conventions (Go, HCL generation, API, React, tests) |
| [07-glossary.md](07-glossary.md) | The project's own vocabulary |
| [08-dev-plan.md](08-dev-plan.md) | Task-level development plan per phase (BE/FE/infra/tests/exit) |
| [09-graph-contract.md](09-graph-contract.md) | Canvas graph JSON shape + HCL resolution rules (BE↔FE contract) |
| [../CLAUDE.md](../CLAUDE.md) | Contributor & AI agent guide (hard rules, ownership, verification) |
| [../samples/README.md](../samples/README.md) | Synthetic IaC samples and the conventions they assume |

## Where things stand

Phases 1-6 are done against `samples/` and a local repo; Phase 7 (security) and
Phase 8 (ADO and production infra) are next. Status lives in
[03-phases.md](03-phases.md) only, and task detail in [08-dev-plan.md](08-dev-plan.md);
do not repeat status elsewhere, link to it.

## Open items

| Item | When |
|---|---|
| DevOps sign-off that the generated `.tf` matches what the team writes by hand (Phase 4 exit) | Before Phase 8 |
| DevOps review of the 9 conventions in `samples/README.md` against the real repos | Before Phase 8 |
| Threat model, secret-handling checklist, identity-scope audit | Phase 7 |
| User manual for DevOps | Phase 7 |
| Federated credential instead of a client secret; Workload Identity; Key Vault; `internal/ado` | Phase 8 |
| OpenAPI code generation and drift check; `golangci-lint` config | Phase 7 |

Closed: module registration (D-015), UI component library and palette (D-026,
D-028), Phase 3 and 4 open questions.
