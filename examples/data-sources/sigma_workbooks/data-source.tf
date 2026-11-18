# All workbooks in the org.
data "sigma_workbooks" "all" {}

output "workbook_names" {
  value = [for w in data.sigma_workbooks.all.workbooks : w.name]
}
