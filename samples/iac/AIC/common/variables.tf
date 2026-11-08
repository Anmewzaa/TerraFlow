variable "subscription_id" {
  description = "Target (spoke) subscription ID."
  type        = string
}

variable "project" {
  description = "Short project code used in resource names."
  type        = string
}

variable "environment" {
  description = "Deployment environment."
  type        = string

  validation {
    condition     = contains(["dev", "uat", "prod"], var.environment)
    error_message = "environment must be dev, uat or prod."
  }
}

variable "location" {
  description = "Azure region."
  type        = string
  default     = "southeastasia"
}

variable "tags" {
  description = "Extra tags merged into common tags."
  type        = map(string)
  default     = {}
}
