# All connections in the org. Useful for resolving a connection's UUID by name.
data "sigma_connections" "all" {}

# Resolve the "Snowflake" connection's id to feed connection grants.
locals {
  snowflake_connection_id = one([
    for c in data.sigma_connections.all.connections : c.id if c.name == "Snowflake"
  ])
}
