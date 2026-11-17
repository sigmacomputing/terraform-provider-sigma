# A version-tag definition, applied to workbook/report versions via the
# Sigma API. Color is one of the names from Sigma's tag UI palette
# (e.g. red, orange, yellow, green, teal, blue, violet, pink, gray).
resource "sigma_tag" "release" {
  name        = "release"
  color       = "violet"
  description = "Production-ready snapshots."
}
