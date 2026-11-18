# See Sigma's docs for the full per-warehouse `details` schema:
# https://help.sigmacomputing.com/reference/createconnection
#
# Snowflake gotcha: `account` should be the raw `<orgname>-<accountname>` form
# (no `.snowflakecomputing.com` suffix — Sigma appends it server-side). Passing
# the FQDN double-appends and the create returns 400 "Could not connect to host".
# `server` is accepted as a synonym for `account`. `host` is NOT — despite the  
# OpenAPI oneOf treating them as interchangeable, Snowflake-typed connections
# reject `host`.
resource "sigma_connection" "warehouse" {
  name = "primary-warehouse"

  details_json = jsonencode({
    type      = "snowflake"
    account   = "MYORG-MYACCOUNT" # raw, no FQDN
    user      = "SIGMA_SVC"
    password  = var.snowflake_password
    warehouse = "ANALYTICS_WH"
    role      = "SIGMA_ROLE"
  })

  # If you ever `terraform import` this resource, uncomment the lifecycle
  # block below — Sigma's GET strips credentials, so the next plan would
  # otherwise propose a re-PATCH of details_json.
  # lifecycle {
  #   ignore_changes = [details_json]
  # }
}

variable "snowflake_password" {
  type      = string
  sensitive = true
}

# Snowflake key-pair auth example (verified end-to-end in stress test):
#
# resource "sigma_connection" "warehouse_kp" {
#   name = "primary-warehouse-kp"
#   details_json = jsonencode({
#     type       = "snowflake"
#     authType   = "key-pair"
#     account    = "MYORG-MYACCOUNT"
#     user       = "SIGMA_SVC"
#     privateKey = var.snowflake_private_key  # PEM-encoded, including BEGIN/END lines
#   })
# }

# Grant a team usage permission on the entire connection.
resource "sigma_connection_grant" "analytics_uses_warehouse" {
  connection_id = sigma_connection.warehouse.id
  team_id       = sigma_team.analytics.id
  permission    = "usage"
}

# Or grant on a specific schema (path).
data "sigma_connection_path" "raw_orders" {
  connection_id = sigma_connection.warehouse.id
  path          = "ANALYTICS_DB.RAW.ORDERS"
}

resource "sigma_connection_path_grant" "raw_orders_team_usage" {
  connection_path_id = data.sigma_connection_path.raw_orders.connection_path_id
  team_id            = sigma_team.analytics.id
  permission         = "usage"
}
