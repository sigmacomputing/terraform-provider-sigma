# All custom plugins in the org (beta API).
data "sigma_plugins" "all" {}

# Resolve a plugin's id by name.
locals {
  my_chart_id = one([
    for p in data.sigma_plugins.all.entries : p.plugin_id if p.name == "My Custom Chart"
  ])
}
