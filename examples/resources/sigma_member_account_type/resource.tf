# For orgs where an IdP (e.g. Okta) creates and deactivates users. The member must
# already exist; destroying this resource demotes them to reset_to (default "Pro").
resource "sigma_member_account_type" "alice" {
  email        = "alice@example.com"
  account_type = "admin"
}

# Import by email.
# terraform import sigma_member_account_type.alice alice@example.com
