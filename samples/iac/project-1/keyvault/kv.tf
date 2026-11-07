module "kv" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//key-vault?ref=v1.4.0"

  name                     = "kv-${local.name_prefix}-01"
  resource_group_name      = data.azurerm_resource_group.app.name
  location                 = data.azurerm_resource_group.app.location
  tenant_id                = data.azurerm_client_config.current.tenant_id
  sku_name                 = "standard"
  purge_protection_enabled = var.environment == "prod"

  private_endpoint = {
    subnet_id            = data.azurerm_subnet.pe.id
    private_dns_zone_ids = [data.azurerm_private_dns_zone.keyvault.id]
  }

  tags = local.common_tags
}
