# Outbound auth Sigma uses when an API connector calls an external HTTP endpoint.
# Distinct from sigma_api_credential (OAuth for Sigma's own REST API).
resource "sigma_api_connector_credential" "example" {
  name      = "example-service-api"
  allowlist = ["*.example.com"]

  # credential_json is write-only: sent on create/update, never read back.
  credential_json = jsonencode({
    authMethod = "basic"
    basic = {
      username = var.service_username
      password = var.service_password
    }
  })
}
