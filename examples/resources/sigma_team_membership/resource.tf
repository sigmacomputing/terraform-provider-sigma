resource "sigma_team_membership" "alice_in_analytics" {
  team_id   = sigma_team.analytics.id
  member_id = sigma_member.alice.id
}
