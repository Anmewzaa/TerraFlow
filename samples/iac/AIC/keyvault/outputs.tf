output "key_vault_id" {
  description = "Key Vault ID."
  value       = module.kv.id
}

output "key_vault_uri" {
  description = "Key Vault URI."
  value       = module.kv.vault_uri
}
