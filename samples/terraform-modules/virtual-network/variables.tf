variable "name" {
  description = "Name of the virtual network. Convention: vnet-<project>-<env>."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group that will contain the virtual network."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "address_space" {
  description = "Address space(s) of the virtual network, in CIDR notation."
  type        = list(string)
}

variable "dns_servers" {
  description = "Custom DNS servers. Leave empty to use Azure-provided DNS."
  type        = list(string)
  default     = []
}

variable "subnets" {
  description = "Subnets to create, keyed by subnet name."
  type = map(object({
    address_prefixes                  = list(string)
    service_endpoints                 = optional(list(string), [])
    private_endpoint_network_policies = optional(string, "Disabled")
    delegation = optional(object({
      name         = string
      service_name = string
      actions      = list(string)
    }))
  }))
  default = {}
}

variable "tags" {
  description = "Tags applied to the virtual network."
  type        = map(string)
  default     = {}
}
