# Terraflow sample IaC

**Synthetic data.** This folder mimics how the DevOps team structures Terraform,
so Terraflow can be developed and tested without real bank code. All names, IDs
and CIDRs are fake. Nothing here is meant to be applied.

These files are used as:

- parser fixtures for the read-only visualizer (Phase 1)
- module schema sources for the module catalog (Phase 3)
- the reference style for the HCL generator's golden files (Phase 4)

## Layout

```
samples/
├── terraform-modules/          # simulates the central module repo (ADO git)
│   ├── resource-group/
│   ├── virtual-network/        # subnets are defined inside the VNet module
│   ├── network-security-group/
│   ├── key-vault/              # includes optional private endpoint
│   └── storage-account/        # includes optional blob private endpoint
└── iac/
    └── project-1/              # one Terraflow "Project" = one repo/folder tree
        ├── common/             # resource groups (network, app, data)
        ├── vnet/               # VNet + subnets + NSGs
        ├── keyvault/           # Key Vault + private endpoint
        └── storage/            # Storage account + private endpoint
```

Each folder under `project-1/` is a **separate root module with its own state**.

## Conventions this sample assumes

DevOps should check these. If the real repo does something differently,
update the sample before Phase 1 starts, because the parser and generator
are built against it.

| # | Assumption | Where |
|---|---|---|
| 1 | One root module per folder, one state file per folder (`project-1/<folder>.tfstate`) | `env/*.backend.hcl` |
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
| Interpolated names from locals and vars | `name = local.rg_names.network` resolves to `rg-proj1-dev-network` with `env/dev.tfvars` |
| Cross-folder `data` lookups | `vnet/data.tf` refers to the RG created in `common/rg.tf` |
| Data lookup with multiple keys | `data "azurerm_subnet" "pe"` matches on name, VNet and RG |
| Same-folder module output references | `module.vnet.subnet_ids["snet-app"]` in `vnet/nsg.tf` |
| Provider alias and external (hub) resources | `data "azurerm_private_dns_zone"` with `provider = azurerm.hub` must be shown as **external**, not as a missing resource |
| Conditional and function expressions | `merge(...)`, `cond ? a : b` |
| Nested object/map inputs | `subnets`, `security_rules`, `private_endpoint` |
| Comments to be preserved | Round-trip tests must keep every comment byte-for-byte |

## Expected cross-folder graph (dev)

```
common/module.rg_network  (rg-proj1-dev-network)
  ├─< vnet/data.azurerm_resource_group.network
  ├─< keyvault/data.azurerm_subnet.pe        (via resource_group_name)
  └─< storage/data.azurerm_subnet.pe         (via resource_group_name)
common/module.rg_app      (rg-proj1-dev-app)
  └─< keyvault/data.azurerm_resource_group.app
common/module.rg_data     (rg-proj1-dev-data)
  └─< storage/data.azurerm_resource_group.data
vnet/module.vnet          (vnet-proj1-dev, subnet snet-pe)
  ├─< keyvault/data.azurerm_subnet.pe
  └─< storage/data.azurerm_subnet.pe
hub (external)
  ├─< keyvault/data.azurerm_private_dns_zone.keyvault
  └─< storage/data.azurerm_private_dns_zone.blob
```

This graph is the acceptance test for Phase 1.

## Validation status

All files parse with `hashicorp/hcl/v2` and are `terraform fmt`-clean
(checked with `hclwrite.Format`). They have **not** been run through
`terraform validate`, because no provider mirror is available.
