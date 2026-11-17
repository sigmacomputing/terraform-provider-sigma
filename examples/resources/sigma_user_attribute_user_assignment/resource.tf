# Override a user-attribute value for a single member.
# Precedence: user assignment > team assignment > sigma_user_attribute.default_value.
resource "sigma_user_attribute_user_assignment" "alice_region" {
  user_attribute_id = sigma_user_attribute.region.id
  user_id           = sigma_member.alice.id
  value             = "eu-west"
}
