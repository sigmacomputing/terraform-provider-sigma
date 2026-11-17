# All user-attribute values assigned to a given team.
data "sigma_team_user_attributes" "engineering" {
  team_id = sigma_team.engineering.id
}
