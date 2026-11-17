data "sigma_saml_service_providers" "all" {}

resource "sigma_saml_service_provider_certificate" "signing" {
  saml_service_provider_id = data.sigma_saml_service_providers.all.ids[0]
  purpose                  = "signing"
  valid_for_days           = 365
  active                   = true
}
