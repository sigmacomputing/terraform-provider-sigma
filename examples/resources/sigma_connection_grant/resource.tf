# Grant a team `usage` on a connection so its members can build workbooks
# that query the warehouse.
resource "sigma_connection_grant" "eng_uses_warehouse" {
  connection_id = sigma_connection.snowflake.id
  team_id       = sigma_team.engineering.id
  permission    = "usage"
}
