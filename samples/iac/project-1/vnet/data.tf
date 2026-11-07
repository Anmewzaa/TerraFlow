# Cross-folder reference: created in ../common/rg.tf
data "azurerm_resource_group" "network" {
  name = local.rg_names.network
}
