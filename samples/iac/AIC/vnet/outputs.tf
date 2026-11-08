output "vnet_id" {
  description = "VNet ID."
  value       = module.vnet.id
}

output "subnet_ids" {
  description = "Subnet IDs keyed by subnet name."
  value       = module.vnet.subnet_ids
}
