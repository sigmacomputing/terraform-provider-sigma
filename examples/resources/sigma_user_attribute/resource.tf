resource "sigma_user_attribute" "region" {
  name          = "region"
  description   = "Region scope for row-level security."
  default_value = "global"
}

resource "sigma_user_attribute_team_assignment" "analytics_region" {
  user_attribute_id = sigma_user_attribute.region.id
  team_id           = sigma_team.analytics.id
  value             = "us-east"
}

resource "sigma_user_attribute_user_assignment" "alice_region" {
  user_attribute_id = sigma_user_attribute.region.id
  user_id           = sigma_member.alice.id
  value             = "eu-west"
}
