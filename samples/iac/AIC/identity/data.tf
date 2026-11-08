# Cross-folder reference: created in ../common/rg.tf
data "azurerm_resource_group" "app" {
  name = local.rg_names.app
}
