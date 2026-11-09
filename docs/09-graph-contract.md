# 09 — Graph contract & HCL resolution rules

**Why this file exists:** the canvas graph is the BE→FE contract and the output
of the hardest code in the project. Without it, every agent re-derives the shape
and the resolution rules by reading `server/internal/hcl/` plus ~30 sample `.tf`
files. Read this instead. The parser implements it (Phase 1) and the golden tests
pin it; keep this file in sync with them.

Keep this in sync with: the parser in `server/internal/hcl/`, the golden
fixtures, and the FE graph renderer.

## Graph JSON shape

The visualizer CLI (`server/cmd/visualize`) and the API (`GET /projects/{id}/graph`) emit this
per project + environment:

```jsonc
{
  "project": "AIC",
  "env": "dev",
  "folders": [
    {
      "name": "common",                 // root-module folder (own state)
      "nodes": [ /* Node */ ],
      "edges": [ /* Edge */ ]
    }
  ],
  "externals": [ /* External */ ]       // referenced but not created here
}
```

### Node
```jsonc
{
  "id": "common.module.rg_network",     // "<folder>.<address>", stable & unique
  "folder": "common",
  "kind": "module",                     // "module" | "data" | "resource"
  "type": "resource-group",             // catalog key for module; TF type for data/resource
  "domain": "neutral",                  // network|compute|data|security|neutral
  "name": "rg-aic-dev-network",       // resolved name, or null
  "nameState": "resolved",              // "resolved" | "unknown"
  "source": "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0",
  "container": null,                    // parent node id for nesting, or null
  "inputs": { },                        // resolved where possible; unresolved -> {"$unknown": true}
  "label": "rg_network",                // module nodes: the block label, to locate it in the file
  "filename": "rg.tf"                   // module nodes: the file that holds it (D-038)
}
```
- `name` is `null` with `nameState:"unknown"` when the expression can't be
  evaluated. **Never** emit `""` or a guessed value.
- `container` drives nesting on the canvas: a subnet's container is its VNet
  node; a resource's container is its resource-group node (matched by the
  resolved `resource_group_name`). `null` = top level in its folder.

### Edge
```jsonc
{ "from": "keyvault.data.azurerm_subnet.pe", "to": "vnet.module.vnet", "kind": "cross-folder-data" }
```
`kind`: `"module-output"` (same-folder reference to another module's output) ·
`"cross-folder-data"` (a `data` source resolving to a producer in another
folder) · `"nesting"` (optional explicit containment edge; `container` already
encodes it).

### External
```jsonc
{ "id": "hub.privatelink.vaultcore.azure.net", "kind": "private_dns_zone",
  "name": "privatelink.vaultcore.azure.net", "source": "hub" }
```
Anything read via a provider alias (e.g. `azurerm.hub`) or otherwise not created
in this repo. Rendered neutral + styled as external; never shown as something
TerraFlow creates.

The parser reads from an `fs.FS` (`BuildGraphFS`, `ImportModulesFS`): a directory, or a snapshot
of a repository held in memory. The result is the same for both (tested on the sample projects).

## HCL resolution rules (what the parser must do)

1. **Parse** every `.tf` in a folder with `hashicorp/hcl/v2` (hclsyntax).
2. **Evaluate** names/values with `go-cty`, in this scope order:
   `env/<env>.tfvars` → `variable` defaults → `locals` (incl. `naming.tf`) →
   functions (`merge`, `format`, conditional `?:`, string interpolation).
3. **Resolve names** from locals: e.g. `name = local.rg_names.network` →
   `rg-aic-dev-network` using `naming.tf` + the env's `project`/`environment`.
4. **unknown** — if any part of an expression can't be evaluated (unknown var, a
   function on unknown input), the value is `unknown`; do not default it.
5. **Cross-folder references** — a `data "azurerm_resource_group" "x"` whose
   resolved `name` equals a producer's resolved name in another folder becomes a
   `cross-folder-data` edge to that producer. The sample repo reconstructs names
   from `naming.tf` locals rather than `terraform_remote_state`; match on the
   reconstructed name.
6. **Subnets** come from the VNet module's `subnets` map (not separate modules) —
   emit one node per subnet with `container` = the VNet node.
7. **Provider alias / hub** — a `data` block with `provider = azurerm.hub` (or
   any alias pointing outside the repo) is an **External**, not a missing node.
8. **Domain** — from the catalog (module key → domain). Until the catalog exists,
   use `server/internal/hcl.DomainForModule`.

## Acceptance (golden)

`samples/iac/AIC` and `samples/iac/AIH` at `env=dev` must produce the graphs in
`samples/README.md` → "Expected cross-folder graph", including every
cross-folder `data` edge and the two hub externals. The output must be identical on
every run (edges are listed in source order, each target once). That tree is the acceptance
list; the golden-file test asserts the serialized JSON byte-for-byte once the
shape here is implemented.

## Importable form (not part of the graph)

`GET /projects/{id}/import` (`hcl.ImportModules`, D-038, D-040) returns the module
calls and plain `resource` blocks as editable canvas nodes with `existing
{folder, filename, label}`. A literal argument becomes a value; a non-literal one
(var, local, module output, conditional, template, `for`) is kept as source text in
`{"$expr": "<text>"}` so editing never freezes a reference. A resource with
`dynamic` or labelled nested blocks is not imported. Shapes are in
`api/openapi.yaml`.
