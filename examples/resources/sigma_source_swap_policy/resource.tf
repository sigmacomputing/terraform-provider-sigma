# Source swap policy that maps paths from a dev connection to a prod connection.
#
# `type` is immutable after creation — changing it forces replacement.
# `swaps_json` is an opaque JSON object whose structure depends on `type`.
# For type "connection", use {"connectionSwaps": [...]}.
resource "sigma_source_swap_policy" "dev_to_prod" {
  type               = "connection"
  name               = "Dev → Prod Swap"
  from_connection_id = "<dev-connection-id>"

  swaps_json = jsonencode({
    connectionSwaps = [
      {
        toConnectionId = "<prod-connection-id>"
        path0          = { value = "ANALYTICS" }
        path1          = { value = "PUBLIC" }
      }
    ]
  })
}
