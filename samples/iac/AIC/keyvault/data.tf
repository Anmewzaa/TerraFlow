data "azurerm_client_config" "current" {}

# Cross-folder reference: created in ../common/rg.tf
data "azurerm_resource_group" "app" {
  name = local.rg_names.app
}

# Cross-folder reference: created in ../vnet/vnet.tf
data "azurerm_subnet" "pe" {
  name                 = "snet-pe"
  virtual_network_name = local.vnet_name
  resource_group_name  = local.rg_names.network
}

# Shared private DNS zone in the hub subscription
data "azurerm_private_dns_zone" "keyvault" {
  provider            = azurerm.hub
  name                = "privatelink.vaultcore.azure.net"
  resource_group_name = var.hub_dns_resource_group_name
}
