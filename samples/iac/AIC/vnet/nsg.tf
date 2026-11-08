module "nsg_app" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//network-security-group?ref=v1.1.0"

  name                = "nsg-${local.name_prefix}-app"
  resource_group_name = data.azurerm_resource_group.network.name
  location            = data.azurerm_resource_group.network.location

  security_rules = [
    {
      name                       = "allow-https-from-vnet"
      priority                   = 100
      direction                  = "Inbound"
      access                     = "Allow"
      protocol                   = "Tcp"
      destination_port_range     = "443"
      source_address_prefix      = "VirtualNetwork"
      destination_address_prefix = "*"
    },
    {
      name                       = "deny-all-inbound"
      priority                   = 4096
      direction                  = "Inbound"
      access                     = "Deny"
      protocol                   = "*"
      destination_port_range     = "*"
      source_address_prefix      = "*"
      destination_address_prefix = "*"
    },
  ]

  # Same-folder reference to another module's output
  subnet_ids = {
    app = module.vnet.subnet_ids["snet-app"]
  }

  tags = local.common_tags
}

module "nsg_pe" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//network-security-group?ref=v1.1.0"

  name                = "nsg-${local.name_prefix}-pe"
  resource_group_name = data.azurerm_resource_group.network.name
  location            = data.azurerm_resource_group.network.location

  subnet_ids = {
    pe = module.vnet.subnet_ids["snet-pe"]
  }

  tags = local.common_tags
}
