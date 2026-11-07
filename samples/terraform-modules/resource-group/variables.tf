variable "name" {
  description = "Name of the resource group. Convention: rg-<project>-<env>-<purpose>."
  type        = string

  validation {
    condition     = can(regex("^rg-[a-z0-9-]{3,60}$", var.name))
    error_message = "Name must start with 'rg-' and contain only lowercase letters, digits and hyphens."
  }
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "southeastasia"
}

variable "lock_level" {
  description = "Management lock on the resource group. Empty string means no lock."
  type        = string
  default     = "CanNotDelete"

  validation {
    condition     = contains(["", "CanNotDelete", "ReadOnly"], var.lock_level)
    error_message = "lock_level must be '', 'CanNotDelete' or 'ReadOnly'."
  }
}

variable "tags" {
  description = "Tags applied to the resource group."
  type        = map(string)
  default     = {}
}
