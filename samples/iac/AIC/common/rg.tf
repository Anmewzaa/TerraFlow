# Resource groups for project-1. Other folders look these up with
# data "azurerm_resource_group" using local.rg_names.

module "rg_network" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name     = local.rg_names.network
  location = var.location
  tags     = local.common_tags
}

module "rg_app" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name       = local.rg_names.app
  location   = var.location
  lock_level = var.environment == "prod" ? "CanNotDelete" : ""
  tags       = local.common_tags
}

module "rg_data" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name     = local.rg_names.data
  location = var.location
  tags     = merge(local.common_tags, { data_classification = "confidential" })
}
