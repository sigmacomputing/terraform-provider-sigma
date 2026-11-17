# Read the permission set of a built-in account type and use it as the
# starting point for a custom one. The account_type_id below is Sigma's
# built-in `view` type — substitute another to start from `explorer`,
# `creator`, or `admin` instead.
data "sigma_account_type_permissions" "view_baseline" {
  account_type_id = "20140502-5fca-4000-8000-000000000004" # built-in `view`
}

resource "sigma_account_type" "view_with_extras" {
  name        = "view-plus"
  description = "View baseline plus a few extra permissions."
  permissions = data.sigma_account_type_permissions.view_baseline.permissions
}
