# List all API connectors in the organization.
data "sigma_api_connectors" "all" {}

output "connector_names" {
  value = [for c in data.sigma_api_connectors.all.connectors : c.name]
}
