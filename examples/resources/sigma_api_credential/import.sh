# Sigma never returns client_secret, so it is empty after import.
# Recreate the credential if you need a usable secret in state.
terraform import sigma_api_credential.example '<client_id>'
