# Give a team "Can use" on an API connector so its members can build Call API
# actions with it. They also need an account type with Trigger API actions.
resource "sigma_api_connector_grant" "oncall" {
  api_connector_id = sigma_api_connector.example.id
  team_id          = sigma_team.oncall.id
}
