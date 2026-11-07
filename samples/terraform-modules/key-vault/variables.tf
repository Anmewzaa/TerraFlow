variable "name" {
  description = "Key Vault name (globally unique, 3-24 chars). Convention: kv-<project>-<env>-<nn>."
  type        = string

  validation {
    condition     = can(regex("^[a-zA-Z][a-zA-Z0-9-]{1,22}[a-zA-Z0-9]$", var.name))
    error_message = "Key Vault name must be 3-24 chars, start with a letter, and contain only letters, digits and hyphens."
  }
}

variable "resource_group_name" {
  description = "Resource group that will contain the Key Vault."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "tenant_id" {
  description = "Entra ID tenant ID used for authenticating requests to the vault."
  type        = string
}

variable "sku_name" {
  description = "Key Vault SKU."
  type        = string
  default     = "standard"

  validation {
    condition     = contains(["standard", "premium"], var.sku_name)
    error_message = "sku_name must be 'standard' or 'premium'."
  }
}

variable "soft_delete_retention_days" {
  description = "Days to retain deleted vaults and objects (7-90)."
  type        = number
  default     = 90

  validation {
    condition     = var.soft_delete_retention_days >= 7 && var.soft_delete_retention_days <= 90
    error_message = "soft_delete_retention_days must be between 7 and 90."
  }
}

variable "purge_protection_enabled" {
  description = "Prevent permanent deletion during the retention period. Required for production."
  type        = bool
  default     = true
}

variable "public_network_access_enabled" {
  description = "Allow access from public networks. Must stay false under bank policy."
  type        = bool
  default     = false
}

variable "private_endpoint" {
  description = "Private endpoint settings. Set to null to skip creating a private endpoint."
  type = object({
    subnet_id            = string
    private_dns_zone_ids = list(string)
  })
  default = null
}

variable "tags" {
  description = "Tags applied to the Key Vault."
  type        = map(string)
  default     = {}
}
