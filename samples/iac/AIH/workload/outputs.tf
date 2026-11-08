output "storage_account_id" {
  description = "Storage account ID."
  value       = module.st_ai.id
}

output "key_vault_uri" {
  description = "Key Vault URI."
  value       = module.kv_ai.vault_uri
}
