module "vnet" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//virtual-network?ref=v2.0.1"

  name                = local.vnet_name
  resource_group_name = data.azurerm_resource_group.network.name
  location            = data.azurerm_resource_group.network.location
  address_space       = var.address_space
  dns_servers         = var.dns_servers

  subnets = {
    "snet-app" = {
      address_prefixes  = [var.subnet_prefixes["app"]]
      service_endpoints = ["Microsoft.KeyVault"]
    }

    # Private endpoints for PaaS services (Key Vault, Storage, ...)
    "snet-pe" = {
      address_prefixes = [var.subnet_prefixes["pe"]]
    }

    "snet-integration" = {
      address_prefixes = [var.subnet_prefixes["integration"]]
      delegation = {
        name         = "appservice"
        service_name = "Microsoft.Web/serverFarms"
        actions      = ["Microsoft.Network/virtualNetworks/subnets/action"]
      }
    }
  }

  tags = local.common_tags
}
