# Let the "hub" tenant deploy content to the "acme" tenant.
resource "sigma_tenant_deployment_capability" "hub_to_acme" {
  tenant_organization_id           = sigma_tenant.hub.id
  deploy_to_tenant_organization_id = sigma_tenant.acme.id
}
