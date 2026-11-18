# List all API-connector credentials. Secrets are never returned.
data "sigma_api_connector_credentials" "all" {}

output "credential_names" {
  value = [for c in data.sigma_api_connector_credentials.all.credentials : c.name]
}
