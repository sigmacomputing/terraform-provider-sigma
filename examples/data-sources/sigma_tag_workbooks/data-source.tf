# All workbooks a given tag has been applied to.
data "sigma_tag_workbooks" "release_workbooks" {
  tag_id = sigma_tag.release.id
}

output "release_count" {
  value = length(data.sigma_tag_workbooks.release_workbooks.workbooks)
}
