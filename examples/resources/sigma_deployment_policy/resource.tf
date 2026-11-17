# version_tag_id is a content-scoped concept (a workbook's version history)
# that this provider does not manage — supply the real ID from Sigma's UI or
# API. It is set once at creation; changing it forces replacement.
resource "sigma_deployment_policy" "prod_rollout" {
  name                  = "Prod Rollout"
  name_in_tenant        = "Production"
  version_tag_id        = "<version-tag-id>"
  source_swap_policies  = [sigma_source_swap_policy.dev_to_prod.id]
  copy_input_table_data = true
}
