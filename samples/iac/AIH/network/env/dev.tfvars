subscription_id = "00000000-0000-0000-0000-000000000011"
project         = "aih"
environment     = "dev"
location        = "southeastasia"

tags = {
  cost_center = "CC-0001"
  owner       = "team-aih"
}

address_space = ["10.40.0.0/22"]
dns_servers   = ["10.0.0.4"]

subnet_prefixes = {
  workload = "10.40.0.0/24"
  pe       = "10.40.1.0/24"
}
