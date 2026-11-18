resource "sigma_api_credential" "ci" {
  owner_id    = sigma_member.alice.id
  name        = "ci-pipeline"
  description = "Used by the CI pipeline to apply Terraform changes."
  scopes      = ["api"]
}

# The secret is only available at create time. Surface it to a downstream
# consumer (a vault, AWS Secrets Manager, etc.) within the same apply.
output "ci_client_id" {
  value = sigma_api_credential.ci.client_id
}

output "ci_client_secret" {
  value     = sigma_api_credential.ci.client_secret
  sensitive = true
}
