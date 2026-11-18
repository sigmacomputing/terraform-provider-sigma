data "sigma_deployment_policy_tenants" "all" {}

output "acme_tenant_id" {
  value = [for t in data.sigma_deployment_policy_tenants.all.tenants : t.tenant_organization_id if t.tenant_organization_slug == "acme"][0]
}
