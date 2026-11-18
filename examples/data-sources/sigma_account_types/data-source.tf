data "sigma_account_types" "all" {}

locals {
  account_type_ids = { for t in data.sigma_account_types.all.entries : t.name => t.id }
}

# Seed a custom type's permissions from the built-in "view" type.
data "sigma_account_type_permissions" "view" {
  account_type_id = local.account_type_ids["view"]
}
