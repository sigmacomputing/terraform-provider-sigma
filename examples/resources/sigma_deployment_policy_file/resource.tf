# Attach a file (workbook, data model, report, or folder inode) to a
# deployment policy. One resource instance per file — use for_each over a
# list of inode IDs to attach many.
resource "sigma_deployment_policy_file" "dashboard" {
  deployment_policy_id = sigma_deployment_policy.prod_rollout.id
  inode_id             = "<file-inode-id>"
}
