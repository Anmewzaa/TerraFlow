# Spoke network for AIH. The resource group and the network share this folder.

module "rg_net" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name     = local.rg_names.network
  location = var.location
  tags     = local.common_tags
}

module "vnet" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//virtual-network?ref=v2.0.1"

  name                = local.vnet_name
  resource_group_name = module.rg_net.name
  location            = module.rg_net.location
  address_space       = var.address_space
  dns_servers         = var.dns_servers

  subnets = {
    "snet-workload" = {
      address_prefixes  = [var.subnet_prefixes["workload"]]
      service_endpoints = ["Microsoft.KeyVault", "Microsoft.Storage"]
    }

    # Private endpoints for PaaS services
    "snet-pe" = {
      address_prefixes = [var.subnet_prefixes["pe"]]
    }
  }

  tags = local.common_tags
}

module "nsg_workload" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//network-security-group?ref=v1.1.0"

  name                = "nsg-${local.name_prefix}-workload"
  resource_group_name = module.rg_net.name
  location            = module.rg_net.location

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
  ]

  # Same-folder reference to another module's output
  subnet_ids = {
    workload = module.vnet.subnet_ids["snet-workload"]
  }

  tags = local.common_tags
}
