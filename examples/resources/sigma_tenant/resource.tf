# Provision a tenant organization for embedded analytics.
#
# `cloud_provider` is immutable after creation — changing it forces replacement.
resource "sigma_tenant" "acme" {
  name           = "Acme Corp"
  slug           = "acme-corp"
  cloud_provider = "aws"
}

# Tenant that inherits the parent org's cloud provider (omit cloud_provider).
resource "sigma_tenant" "beta_customer" {
  name = "Beta Customer"
  slug = "beta-customer"
}
