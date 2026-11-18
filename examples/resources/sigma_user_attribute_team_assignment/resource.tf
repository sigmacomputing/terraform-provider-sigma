# Pin a user-attribute value for every member of a team.
# A user-level assignment (sigma_user_attribute_user_assignment) overrides this
# for individual members.
resource "sigma_user_attribute_team_assignment" "eng_region" {
  user_attribute_id = sigma_user_attribute.region.id
  team_id           = sigma_team.engineering.id
  value             = "us-east"
}
