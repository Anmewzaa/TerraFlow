variable "name" {
  description = "Name of the network security group. Convention: nsg-<project>-<env>-<subnet>."
  type        = string
}

variable "resource_group_name" {
  description = "Resource group that will contain the NSG."
  type        = string
}

variable "location" {
  description = "Azure region."
  type        = string
}

variable "security_rules" {
  description = "Security rules. Priority must be unique per direction (100-4096)."
  type = list(object({
    name                       = string
    priority                   = number
    direction                  = string # Inbound | Outbound
    access                     = string # Allow | Deny
    protocol                   = string # Tcp | Udp | Icmp | *
    source_port_range          = optional(string, "*")
    destination_port_range     = string
    source_address_prefix      = string
    destination_address_prefix = string
  }))
  default = []
}

variable "subnet_ids" {
  description = "Subnets to associate with this NSG, keyed by a static label."
  type        = map(string)
  default     = {}
}

variable "tags" {
  description = "Tags applied to the NSG."
  type        = map(string)
  default     = {}
}
