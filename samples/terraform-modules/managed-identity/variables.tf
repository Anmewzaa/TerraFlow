variable "name" {
  description = "Identity name. Convention: id-<project>-<env>-<purpose>."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group that will contain the identity."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "tags" {
  description = "Tags applied to the identity."
  type        = map(string)
  default     = {}
}
