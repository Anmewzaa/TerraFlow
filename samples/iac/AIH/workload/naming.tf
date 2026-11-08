# Naming convention shared by every folder in this project.
# Keep this file identical across folders: cross-folder data lookups
# rebuild resource names from these locals instead of using remote state.
locals {
  name_prefix = "${var.project}-${var.environment}"

  rg_names = {
    network  = "rg-${local.name_prefix}-network"
    workload = "rg-${local.name_prefix}-workload"
  }

  vnet_name = "vnet-${local.name_prefix}"

  common_tags = merge(var.tags, {
    project     = var.project
    environment = var.environment
    managed_by  = "terraform"
  })
}
