# Target a tenant org for this deployment policy. Look up
# tenant_organization_id via data.sigma_deployment_policy_tenants.
resource "sigma_deployment_policy_tenant" "acme" {
  deployment_policy_id   = sigma_deployment_policy.prod_rollout.id
  tenant_organization_id = data.sigma_deployment_policy_tenants.all.tenants[0].tenant_organization_id
}
