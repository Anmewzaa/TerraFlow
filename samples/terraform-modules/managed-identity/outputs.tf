output "id" {
  description = "Identity resource ID."
  value       = azurerm_user_assigned_identity.this.id
}

output "principal_id" {
  description = "Identity principal (object) ID."
  value       = azurerm_user_assigned_identity.this.principal_id
}

output "client_id" {
  description = "Identity client ID."
  value       = azurerm_user_assigned_identity.this.client_id
}
