# Project settings: where generated HCL goes, and how names become variables

Status: **built (2026-11-10): migration 016, `GET/PUT /projects/{id}/generation`, project variables, `variables.tf`, `env/<env>.tfvars`, starter files, layout defaults, the Settings screen and the Advanced step; see D-057 for what differs from this note.** Original design note: It is the first part of M3 (docs/03-phases.md) and
the answer to the request for advanced settings before a project is created. Read
`docs/09-graph-contract.md` and `samples/README.md` first.

## What exists today

- Every canvas node already carries `target_folder` and `filename` (`hcl.GenerateNode`). The generator
  groups nodes by folder and writes each module call to `filename` (default `main.tf`). The catalog gives a
  module a default `target_folder`; the canvas lets a person change both per node (`FileTargetFields`).
- The parser reads variables, locals and `env/<env>.tfvars` to resolve names (`hcl/eval.go`). The generator
  **never writes** `.tfvars` or `variables.tf`: a value typed on the canvas is written as a literal in the
  module call.
- Nothing is stored per project about layout. Every person re-decides folder and file for every node.

## What the request asks for

1. **One file**: all generated HCL in a single file, for example `terraform/main.tf`.
2. **Per-folder layout**: for example `terraform/common/main.tf` and `terraform/network/main.tf`, and, where
   wanted, the file name chosen per kind of resource (`rg.tf`).
3. **Variables**: when a person types a resource name, TerraFlow maps it to a variable and writes the value into
   an `.tfvars` file when it generates the HCL.
4. Further suggestions (below).

## Proposal: a layout policy stored on the project

One JSON document per project (a new `projects.settings` column, migration 016, additive), chosen in an
"Advanced" step of the project dialog and editable later by the owner and co-owners. It supplies the
**defaults** for `target_folder` and `filename`; a person can still override them on a node.

```json
{
  "layout": {
    "mode": "single | per_folder",
    "root": "terraform",                      // every path is relative to the project folder
    "single_file": "main.tf",                 // mode single
    "folders": { "azurerm_resource_group": "common", "virtual-network": "network" },   // mode per_folder
    "files":   { "azurerm_resource_group": "rg.tf" },                                  // optional, by kind
    "default_file": "main.tf"
  },
  "variables": {
    "mode": "literal | tfvars",
    "tfvars_file": "env/{env}.tfvars",
    "declare": true                           // also write the variable blocks that are missing
  }
}
```

Rules that keep it safe:

- Paths and names pass the existing validators (`validFolder`, `validFilename`); the policy is data, never code.
- The policy fills the node defaults **before** generation, so the generator and its golden tests do not change
  for projects without a policy (the default is today's behaviour: `literal`, the module's own folder, `main.tf`).
- A node that edits an existing module call keeps the folder, file and label the repository gives it
  (`Existing`), whatever the policy says.
- Hard rules 5 and 6 are untouched: module calls by default, raw resources only as explicit raw nodes.

## Variables (mode `tfvars`)

When the mode is on, a value that a person typed for a field becomes `var.<name>` in the module call and a line
in `env/<env>.tfvars`. Open questions that need an answer before coding:

1. **Naming.** `<instance>_<field>` (for example `rg_main_name`) is predictable and unique; the team's own
   convention (samples/README.md) may differ. Proposal: a template in the policy, `{instance}_{field}`.
2. **Which fields.** Every field, or only the ones the person marks (names, locations, sizes)? Proposal: names
   and fields marked `x-tfvar` in the module schema, so a schema author decides.
3. **Where the declaration goes.** `variables.tf` in the folder, appended; never rewriting an existing block
   (round-trip tests must still show untouched bytes unchanged).
4. **Environments.** One `.tfvars` per environment already exists in the contract (`env/<env>.tfvars`); a value
   typed while the canvas shows `dev` writes only `dev`; the other environments get no line and the reviewer is
   told (honest state, hard rule 8).
5. **Existing values.** If the variable already exists with a different value, the change is shown as an edit,
   never silently overwritten.

## Suggestions beyond the request

- **Naming template** for instances and file names (`{kind}-{env}`), so a team's names are not retyped.
- **Preview of the layout** in the dialog: the tree of files that a few sample nodes would create.
- **Backend and provider files**: an optional starter (`providers.tf`, `backend.tf`) written on the first publish
  of a new folder (D-054 left starter files open), from a template the owner edits.
- **Per-environment folders** (`terraform/dev`, `terraform/prod`) as a mode next to `single` and `per_folder`.
- **Import the layout from the repository**: read the existing folders and file names and propose the policy.

## Order of work

1. Migration 016 and the policy type with validation, and the defaults applied to nodes (no generator change).
2. The "Advanced" step in the project dialog and a settings page for owners.
3. `variables.mode = tfvars`: generator support with golden and round-trip tests first, then the UI.
4. The catalog scope work of M3 builds on the same `projects.settings` column.

Decision needed before step 3: the naming template and which fields become variables (questions 1 and 2).
