# All members in the org. Useful for `for_each` lookups by email.
data "sigma_members" "all" {}

output "all_emails" {
  value = [for m in data.sigma_members.all.members : m.email]
}
