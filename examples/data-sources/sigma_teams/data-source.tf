# All teams in the org.
data "sigma_teams" "all" {}

output "team_names" {
  value = [for t in data.sigma_teams.all.teams : t.name]
}
