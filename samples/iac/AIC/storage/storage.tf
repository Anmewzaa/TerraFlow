module "st_data" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//storage-account?ref=v1.3.2"

  # Storage names allow only lowercase letters and digits
  name                     = "st${var.project}${var.environment}01"
  resource_group_name      = data.azurerm_resource_group.data.name
  location                 = data.azurerm_resource_group.data.location
  account_replication_type = var.environment == "prod" ? "GZRS" : "ZRS"
  containers               = ["documents", "exports"]

  private_endpoint = {
    subnet_id            = data.azurerm_subnet.pe.id
    private_dns_zone_ids = [data.azurerm_private_dns_zone.blob.id]
  }

  tags = local.common_tags
}
