# Create a top-level folder in the org root.
resource "sigma_folder" "reports" {
  name        = "Reports"
  description = "Shared reporting folder"
}

# Create a nested folder inside an existing workspace or folder.
resource "sigma_folder" "team_reports" {
  name      = "Team Reports"
  parent_id = "<workspace-or-folder-id>"
}
