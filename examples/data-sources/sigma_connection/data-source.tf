# Look up a single connection by name (set exactly one of name / id).
data "sigma_connection" "snowflake" {
  name = "Snowflake"
}

# Use the resolved id, e.g. for a connection grant on an unmanaged connection.
resource "sigma_connection_grant" "analysts" {
  connection_id = data.sigma_connection.snowflake.id
  team_id       = sigma_team.analysts.id
  permission    = "usage"
}
