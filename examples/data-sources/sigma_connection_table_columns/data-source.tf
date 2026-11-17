# The column list for a Sigma-tracked connection table. Useful when you need
# to code-generate workbook column references in larger automation pipelines.
data "sigma_connection_table_columns" "customers" {
  table_id = "<connection-table-id-here>"
}
