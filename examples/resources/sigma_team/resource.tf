resource "sigma_team" "analytics" {
  name        = "analytics"
  description = "Analytics team — owns the data-warehouse-side model."
  visibility  = "private"
}
