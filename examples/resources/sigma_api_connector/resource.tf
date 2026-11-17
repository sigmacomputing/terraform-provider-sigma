# An API connector defines how Sigma calls an external HTTP endpoint from within
# a workbook. The url host must match the credential's allowlist.
resource "sigma_api_connector" "example" {
  name        = "example-data-api"
  description = "Fetches reference data from the example service"
  auth_id     = sigma_api_connector_credential.example.id

  params_json = jsonencode({
    method      = "GET"
    url         = "https://api.example.com/v1/data"
    headers     = []
    pathParams  = []
    queryParams = []
    body        = ""
  })
}
