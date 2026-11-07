# Cross-folder reference: created in ../common/rg.tf
data "azurerm_resource_group" "data" {
  name = local.rg_names.data
}

# Cross-folder reference: created in ../vnet/vnet.tf
data "azurerm_subnet" "pe" {
  name                 = "snet-pe"
  virtual_network_name = local.vnet_name
  resource_group_name  = local.rg_names.network
}

# Shared private DNS zone in the hub subscription
data "azurerm_private_dns_zone" "blob" {
  provider            = azurerm.hub
  name                = "privatelink.blob.core.windows.net"
  resource_group_name = var.hub_dns_resource_group_name
}
