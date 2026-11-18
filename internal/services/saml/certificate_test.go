package saml_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Gated on SIGMA_TEST_SAML_SP_ID — needs an org with a SAML service provider (Beta API).
func TestAccSAMLServiceProviderCertificate_basic(t *testing.T) {
	spID := os.Getenv("SIGMA_TEST_SAML_SP_ID")
	if spID == "" {
		t.Skip("SIGMA_TEST_SAML_SP_ID not set; skipping SAML certificate test")
	}
	cfg := func(active bool) string {
		return fmt.Sprintf(`
data "sigma_saml_service_providers" "all" {}

resource "sigma_saml_service_provider_certificate" "test" {
  saml_service_provider_id = %q
  purpose                  = "signing"
  valid_for_days           = 30
  active                   = %t
}
`, spID, active)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_saml_service_provider_certificate.test", "active", "false"),
					resource.TestCheckResourceAttrSet("sigma_saml_service_provider_certificate.test", "public_key_certificate"),
					resource.TestCheckResourceAttrSet("data.sigma_saml_service_providers.all", "ids.#"),
				),
			},
			{
				Config: cfg(true),
				Check:  resource.TestCheckResourceAttr("sigma_saml_service_provider_certificate.test", "active", "true"),
			},
			{
				ResourceName:            "sigma_saml_service_provider_certificate.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"valid_for_days"},
			},
		},
	})
}
