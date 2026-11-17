# A signing key set for a JWT bearer (OAuth) warehouse connection.
resource "sigma_jwk_set" "snowflake" {
  valid_for_days = 365
}

# Register this certificate with the warehouse / identity provider.
output "snowflake_jwt_public_key" {
  value = sigma_jwk_set.snowflake.public_key
}

# Reference the set from the connection's details.
resource "sigma_connection" "snowflake" {
  name = "snowflake-oauth"
  details_json = jsonencode({
    type      = "snowflake"
    account   = "MYORG-MYACCOUNT"
    warehouse = "ANALYTICS_WH"
    useOauth  = true
    useJwt    = true
    jwkSetId  = sigma_jwk_set.snowflake.id
    # ...other OAuth fields per Sigma's connection schema
  })
}
