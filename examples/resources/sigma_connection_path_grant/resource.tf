# Grant a team `usage` on a sub-tree (database/schema/table) of a connection.
# Path UUIDs are resolved via data.sigma_connection_path from a DOT-joined
# warehouse path (DATABASE.SCHEMA.TABLE) that matches a synced catalog path.
data "sigma_connection_path" "customers_table" {
  connection_id = sigma_connection.snowflake.id
  path          = "ANALYTICS.PUBLIC.CUSTOMERS"
}

resource "sigma_connection_path_grant" "analysts_can_query_customers" {
  connection_path_id = data.sigma_connection_path.customers_table.id
  team_id            = sigma_team.analysts.id
  permission         = "usage"
}
