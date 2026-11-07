subscription_id = "00000000-0000-0000-0000-000000000003"
project         = "proj1"
environment     = "prod"
location        = "southeastasia"

tags = {
  cost_center = "CC-0000"
  owner       = "team-proj1"
}

address_space = ["10.30.0.0/22"]
dns_servers   = ["10.0.0.4", "10.0.0.5"]

subnet_prefixes = {
  app         = "10.30.0.0/24"
  pe          = "10.30.1.0/24"
  integration = "10.30.2.0/26"
}
