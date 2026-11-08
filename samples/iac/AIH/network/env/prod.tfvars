subscription_id = "00000000-0000-0000-0000-000000000012"
project         = "aih"
environment     = "prod"
location        = "southeastasia"

tags = {
  cost_center = "CC-0001"
  owner       = "team-aih"
}

address_space = ["10.50.0.0/22"]
dns_servers   = ["10.0.0.4"]

subnet_prefixes = {
  workload = "10.50.0.0/24"
  pe       = "10.50.1.0/24"
}
