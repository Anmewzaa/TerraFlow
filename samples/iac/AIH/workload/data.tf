data "azurerm_client_config" "current" {}

# Cross-folder reference: created in ../network/main.tf
data "azurerm_subnet" "pe" {
  name                 = "snet-pe"
  virtual_network_name = local.vnet_name
  resource_group_name  = local.rg_names.network
}

# Shared private DNS zones in the hub subscription
data "azurerm_private_dns_zone" "blob" {
  provider            = azurerm.hub
  name                = "privatelink.blob.core.windows.net"
  resource_group_name = var.hub_dns_resource_group_name
}

data "azurerm_private_dns_zone" "keyvault" {
  provider            = azurerm.hub
  name                = "privatelink.vaultcore.azure.net"
  resource_group_name = var.hub_dns_resource_group_name
}
