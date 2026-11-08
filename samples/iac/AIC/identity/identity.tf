# Identity used by the application to read secrets and blobs.
module "id_app" {
  source = "git::https://dev.azure.com/contoso/platform/_git/terraform-modules//managed-identity?ref=v1.0.0"

  name                = "id-${local.name_prefix}-app"
  resource_group_name = data.azurerm_resource_group.app.name
  location            = data.azurerm_resource_group.app.location

  tags = local.common_tags
}
