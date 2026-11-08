terraform {
  required_version = ">= 1.6.0"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.20"
    }
  }

  # Backend values are injected by the pipeline:
  #   terraform init -backend-config=env/<env>.backend.hcl
  backend "azurerm" {}
}

provider "azurerm" {
  features {}
  subscription_id = var.subscription_id
}

# Hub subscription holds the shared private DNS zones (read-only lookups).
provider "azurerm" {
  alias = "hub"
  features {}
  subscription_id = var.hub_subscription_id
}
