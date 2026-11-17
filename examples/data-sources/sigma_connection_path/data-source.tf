# Resolve a slash-delimited warehouse path into the path UUID needed by
# sigma_connection_path_grant.
data "sigma_connection_path" "customers_table" {
  connection_id = sigma_connection.snowflake.id
  path          = "ANALYTICS/PUBLIC/CUSTOMERS"
}
