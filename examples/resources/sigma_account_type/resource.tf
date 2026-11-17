# Look up the available permissions for a known reference account type.
# Pass the resulting names through to your custom account type.
data "sigma_account_type_permissions" "creator" {
  account_type_id = "<creator-account-type-uuid>"
}

resource "sigma_account_type" "limited_creator" {
  name        = "limited-creator"
  description = "Creators restricted from sharing externally."
  permissions = setsubtract(
    data.sigma_account_type_permissions.creator.permissions,
    ["share_workbook_externally"],
  )
}
