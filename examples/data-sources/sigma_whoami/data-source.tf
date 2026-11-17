data "sigma_whoami" "current" {}

# Fail the plan if the credentials point at the wrong org.
resource "terraform_data" "org_guard" {
  lifecycle {
    precondition {
      condition     = data.sigma_whoami.current.organization_id == var.expected_org_id
      error_message = "Sigma credentials belong to the wrong organization."
    }
  }
}

variable "expected_org_id" {
  type = string
}
