output "resource_group_names" {
  description = "Resource group names created in this folder."
  value = {
    network = module.rg_network.name
    app     = module.rg_app.name
    data    = module.rg_data.name
  }
}
