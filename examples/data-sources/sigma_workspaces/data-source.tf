# All workspaces in the org.
data "sigma_workspaces" "all" {}

output "workspace_names" {
  value = [for w in data.sigma_workspaces.all.workspaces : w.name]
}
