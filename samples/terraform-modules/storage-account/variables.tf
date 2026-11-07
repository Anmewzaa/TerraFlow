variable "name" {
  description = "Storage account name (globally unique). Convention: st<project><env><nn>."
  type        = string

  validation {
    condition     = can(regex("^[a-z0-9]{3,24}$", var.name))
    error_message = "Storage account name must be 3-24 lowercase letters or digits."
  }
}

variable "resource_group_name" {
  description = "Resource group that will contain the storage account."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "account_tier" {
  description = "Performance tier."
  type        = string
  default     = "Standard"

  validation {
    condition     = contains(["Standard", "Premium"], var.account_tier)
    error_message = "account_tier must be 'Standard' or 'Premium'."
  }
}

variable "account_replication_type" {
  description = "Replication type."
  type        = string
  default     = "ZRS"

  validation {
    condition     = contains(["LRS", "ZRS", "GRS", "RAGRS", "GZRS", "RAGZRS"], var.account_replication_type)
    error_message = "Invalid account_replication_type."
  }
}

variable "public_network_access_enabled" {
  description = "Allow access from public networks. Must stay false under bank policy."
  type        = bool
  default     = false
}

variable "containers" {
  description = "Private blob containers to create."
  type        = list(string)
  default     = []
}

variable "private_endpoint" {
  description = "Private endpoint (blob) settings. Set to null to skip creating a private endpoint."
  type = object({
    subnet_id            = string
    private_dns_zone_ids = list(string)
  })
  default = null
}

variable "tags" {
  description = "Tags applied to the storage account."
  type        = map(string)
  default     = {}
}
