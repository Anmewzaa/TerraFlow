# Application workload for AIH: storage and Key Vault behind private endpoints.

module "rg_workload" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//resource-group?ref=v1.2.0"

  name       = local.rg_names.workload
  location   = var.location
  lock_level = var.environment == "prod" ? "CanNotDelete" : ""
  tags       = local.common_tags
}

module "st_ai" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//storage-account?ref=v1.3.2"

  # Storage names allow only lowercase letters and digits
  name                     = "st${var.project}${var.environment}01"
  resource_group_name      = module.rg_workload.name
  location                 = module.rg_workload.location
  account_replication_type = var.environment == "prod" ? "GZRS" : "LRS"
  containers               = ["models", "datasets"]

  private_endpoint = {
    subnet_id            = data.azurerm_subnet.pe.id
    private_dns_zone_ids = [data.azurerm_private_dns_zone.blob.id]
  }

  tags = merge(local.common_tags, { data_classification = "internal" })
}

module "kv_ai" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//key-vault?ref=v1.4.0"

  name                     = "kv-${local.name_prefix}-01"
  resource_group_name      = module.rg_workload.name
  location                 = module.rg_workload.location
  tenant_id                = data.azurerm_client_config.current.tenant_id
  sku_name                 = "standard"
  purge_protection_enabled = var.environment == "prod"

  private_endpoint = {
    subnet_id            = data.azurerm_subnet.pe.id
    private_dns_zone_ids = [data.azurerm_private_dns_zone.keyvault.id]
  }

  tags = local.common_tags
}
