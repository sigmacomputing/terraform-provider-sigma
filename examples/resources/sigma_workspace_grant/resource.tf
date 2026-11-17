# Grant a team `view` on a workspace. Members of the team see every workbook
# inside the workspace at this permission level (or higher, if a per-workbook
# grant exists). Permission options: view, explore, edit, account.
resource "sigma_workspace_grant" "analytics_engineering" {
  workspace_id = sigma_workspace.analytics.id
  team_id      = sigma_team.engineering.id
  permission   = "view"
}
