# Manage a subset of org-wide settings. Unconfigured blocks (csv_upload and
# license_upgrade_requests here) are left untouched — Terraform reflects
# Sigma's current value for them instead of erroring or drifting, so you can
# adopt settings piecemeal.
resource "sigma_organization_settings" "this" {
  timezone = {
    timezone = "America/Los_Angeles"
  }

  comments = {
    enabled                   = true
    allow_annotations         = true
    annotation_retention_days = 365
  }

  audit_logging = {
    enabled = true
  }

  bulk_copy = {
    enabled = false
  }

  public_embeds = {
    enabled = false
  }

  sample_connections = {
    enable_snowflake  = false
    enable_databricks = false
  }
}
