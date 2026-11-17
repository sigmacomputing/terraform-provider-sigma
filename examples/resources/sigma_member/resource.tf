# member_type is the account type's canonical license name. Use the value Sigma
# stores (e.g. "build", "view", "admin") — note Sigma normalizes some UI labels
# (the "Creator" account type is stored as "build"), so passing "creator" here
# causes an "inconsistent result after apply" error. Use "build".
resource "sigma_member" "alice" {
  email       = "alice@example.com"
  first_name  = "Alice"
  last_name   = "Anderson"
  member_type = "build"
  user_kind   = "internal"
}

# Destroying this resource deactivates Alice (Sigma retains audit history);
# it is not a hard delete.

# Silent provisioning: skip the invitation email at create time, e.g. when an
# external IdP (Okta/Azure AD) handles user onboarding.
resource "sigma_member" "bot" {
  email       = "svc-bot@example.com"
  first_name  = "Service"
  last_name   = "Bot"
  member_type = "build"
  send_invite = false
}
