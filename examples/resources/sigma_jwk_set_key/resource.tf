# Rotation: stage a new key (inactive), register its public_key with the IdP,
# then set active = true. The previous key stays, inactive, until deleted.
resource "sigma_jwk_set_key" "next" {
  jwk_set_id     = sigma_jwk_set.snowflake.id
  valid_for_days = 365
  active         = false
}

# To delete the set's original key after rotating, adopt it, apply, then remove
# this block and apply again:
import {
  to = sigma_jwk_set_key.original
  id = "${sigma_jwk_set.snowflake.id}:<original-key-id>"
}

resource "sigma_jwk_set_key" "original" {
  jwk_set_id = sigma_jwk_set.snowflake.id
}
