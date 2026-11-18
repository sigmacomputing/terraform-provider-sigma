resource "sigma_user_attribute_tenant_assignment" "acme" {
  user_attribute_id      = sigma_user_attribute.region.id
  tenant_organization_id = sigma_tenant.acme.id
  value                  = "emea"
}
