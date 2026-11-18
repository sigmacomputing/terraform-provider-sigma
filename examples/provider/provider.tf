terraform {
  required_providers {
    sigma = {
      source  = "sigmacomputing/sigma"
      version = "~> 0.15"
    }
  }
}

# All three attributes can be set via env vars instead:
#   SIGMA_BASE_URL, SIGMA_CLIENT_ID, SIGMA_CLIENT_SECRET
provider "sigma" {
  base_url      = "https://api.sigmacomputing.com"
  client_id     = var.sigma_client_id
  client_secret = var.sigma_client_secret
}

variable "sigma_client_id" {
  type      = string
  sensitive = true
}

variable "sigma_client_secret" {
  type      = string
  sensitive = true
}
