# All teams a given member belongs to.
data "sigma_member_teams" "alice_teams" {
  member_id = sigma_member.alice.id
}

output "alice_team_count" {
  value = length(data.sigma_member_teams.alice_teams.teams)
}
