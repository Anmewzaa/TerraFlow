# Terraflow sample IaC

**Synthetic data.** This folder mimics how the DevOps team structures Terraform,
so Terraflow can be developed and tested without real bank code. All names, IDs
and CIDRs are fake. Nothing here is meant to be applied.

These files are used as:

- parser fixtures for the visualizer and the importer
- module schema sources for the module catalog
- the reference style for the HCL generator's golden and merge tests

## Layout

```
samples/
├── terraform-modules/          # simulates the central module repo (ADO git)
│   ├── resource-group/
│   ├── virtual-network/        # subnets are defined inside the VNet module
│   ├── network-security-group/
│   ├── key-vault/              # includes optional private endpoint
│   ├── managed-identity/       # user-assigned identity
│   └── storage-account/        # includes optional blob private endpoint
└── iac/                        # two mock projects, each "Project" = one repo/folder tree
    ├── AIC/                    # many small folders, one file per resource kind
    │   ├── common/             # resource groups (network, app, data)
    │   ├── vnet/               # VNet + subnets + NSGs
    │   ├── keyvault/           # Key Vault + private endpoint
    │   ├── storage/            # Storage account + private endpoint
    │   └── identity/           # app managed identity
    └── AIH/                    # fewer folders, everything in main.tf
        ├── network/            # resource group + VNet + NSG in one folder
        └── workload/           # resource group + storage + Key Vault
```

AIC and AIH differ on purpose so the importer and generator are tested on more
than one layout: AIC uses one file per resource kind and looks resource groups up
with cross-folder `data` sources; AIH keeps each resource group next to its
resources (same-folder module outputs) and has a single cross-folder lookup.

Each folder under a project is a **separate root module with its own state**.

## Conventions this sample assumes

DevOps should check these. If the real repo does something differently,
update the sample before Phase 8 (real repositories), because the parser and
generator are built against it.

| # | Assumption | Where |
|---|---|---|
| 1 | One root module per folder, one state file per folder (`<project>/<folder>.tfstate`) | `env/*.backend.hcl` |
| 2 | Environments are selected by the pipeline: `-var-file=env/<env>.tfvars` and `-backend-config=env/<env>.backend.hcl` | `env/` |
| 3 | Names come from a shared `naming.tf` (`locals`), which is copied identically into every folder | `*/naming.tf` |
| 4 | **Cross-folder references use `data` sources** that rebuild the target name from `naming.tf` locals. No `terraform_remote_state` | `vnet/data.tf`, `keyvault/data.tf`, `storage/data.tf` |
| 5 | Resources are created only through **module calls**. Modules are sourced from ADO git with a version tag: `git::https://dev.azure.com/<org>/<project>/_git/terraform-modules//<module>?ref=vX.Y.Z` | `*.tf` |
| 6 | Subnets are part of the VNet module (`subnets` map), not separate module calls | `vnet/vnet.tf` |
| 7 | Shared private DNS zones live in a **hub subscription**, read via a provider alias `azurerm.hub` | `*/providers.tf`, `keyvault/data.tf` |
| 8 | Private endpoints are created **inside** the resource module (`private_endpoint` input), not as standalone modules | `key-vault`, `storage-account` |
| 9 | Environment-dependent settings use conditional expressions (`var.environment == "prod" ? ... : ...`) | `common/rg.tf`, `storage/storage.tf` |

## Cases the parser must handle

| Case | Example |
|---|---|
| Interpolated names from locals and vars | `name = local.rg_names.network` resolves to `rg-aic-dev-network` with `env/dev.tfvars` |
| Cross-folder `data` lookups | `vnet/data.tf` refers to the RG created in `common/rg.tf` |
| Data lookup with multiple keys | `data "azurerm_subnet" "pe"` matches on name, VNet and RG |
| Same-folder module output references | `module.vnet.subnet_ids["snet-app"]` in `vnet/nsg.tf` |
| Provider alias and external (hub) resources | `data "azurerm_private_dns_zone"` with `provider = azurerm.hub` must be shown as **external**, not as a missing resource |
| Conditional and function expressions | `merge(...)`, `cond ? a : b` |
| Nested object/map inputs | `subnets`, `security_rules`, `private_endpoint` |
| Comments to be preserved | Round-trip tests must keep every comment byte-for-byte |

## Expected cross-folder graph (AIC, dev)

```
common/module.rg_network  (rg-aic-dev-network)
  ├─< vnet/data.azurerm_resource_group.network
  ├─< keyvault/data.azurerm_subnet.pe        (via resource_group_name)
  └─< storage/data.azurerm_subnet.pe         (via resource_group_name)
common/module.rg_app      (rg-aic-dev-app)
  ├─< identity/data.azurerm_resource_group.app
  └─< keyvault/data.azurerm_resource_group.app
common/module.rg_data     (rg-aic-dev-data)
  └─< storage/data.azurerm_resource_group.data
vnet/module.vnet          (vnet-aic-dev, subnet snet-pe)
  ├─< keyvault/data.azurerm_subnet.pe
  └─< storage/data.azurerm_subnet.pe
hub (external)
  ├─< keyvault/data.azurerm_private_dns_zone.keyvault
  └─< storage/data.azurerm_private_dns_zone.blob
```

Expected graph (AIH, dev):

```
network/subnet.snet-pe    (vnet-aih-dev)
  └─< workload/data.azurerm_subnet.pe
network/module.vnet ──> network/module.rg_net      (same-folder module outputs)
network/module.nsg_workload ──> module.vnet, module.rg_net
hub (external)
  ├─< workload/data.azurerm_private_dns_zone.keyvault
  └─< workload/data.azurerm_private_dns_zone.blob
```

These graphs are pinned by golden files in `server/internal/hcl/testdata/`
(`aic-dev`, `aic-prod`, `aih-dev`)

## Validation status

All files parse with `hashicorp/hcl/v2` and are `terraform fmt`-clean; both are
checked on every test run (`TestSamplesAreFormattedAndSynthetic`). The generator is
also tested against every file and every module call in both projects
(`merge_samples_test.go`): adding a resource keeps existing bytes, and editing a
module changes only its own block. They have **not** been run through
`terraform validate`, because no provider mirror is available.
