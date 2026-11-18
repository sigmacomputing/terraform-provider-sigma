# A Sigma custom plugin (beta API).
#
# Sigma loads the plugin from `url` when it is added to a workbook, and from
# `dev_url` when the plugin runs in development mode. Both default server-side
# when omitted (url to empty, dev_url to http://localhost:5173).
resource "sigma_plugin" "my_chart" {
  name        = "My Custom Chart"
  description = "A bespoke visualization element."
  url         = "https://plugins.example.com/my-chart"
  dev_url     = "http://localhost:5173"
}
