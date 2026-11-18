package apiconnectors_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccAPIConnectorCredential_basic covers create -> update (name/description/
// allowlist) -> import. credential_json is write-only (never returned by GET), so
// it's ignored on import-verify.
func TestAccAPIConnectorCredential_basic(t *testing.T) {
	name := acctest.AccRandName("apicred")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialConfig(name, "initial", `["*.example.com"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_api_connector_credential.test", "name", name),
					resource.TestCheckResourceAttr("sigma_api_connector_credential.test", "description", "initial"),
					resource.TestCheckResourceAttr("sigma_api_connector_credential.test", "allowlist.#", "1"),
					resource.TestCheckResourceAttr("sigma_api_connector_credential.test", "auth_method", "basic"),
					resource.TestCheckResourceAttrSet("sigma_api_connector_credential.test", "id"),
				),
			},
			{
				Config: testAccCredentialConfig(name, "updated", `["*.example.com", "api.internal.net"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_api_connector_credential.test", "description", "updated"),
					resource.TestCheckResourceAttr("sigma_api_connector_credential.test", "allowlist.#", "2"),
				),
			},
			{
				ResourceName:            "sigma_api_connector_credential.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credential_json"}, // write-only; not returned by GET
			},
		},
	})
}

func testAccCredentialConfig(name, description, allowlist string) string {
	return fmt.Sprintf(`
resource "sigma_api_connector_credential" "test" {
  name        = %q
  description = %q
  allowlist   = %s
  credential_json = jsonencode({
    authMethod = "basic"
    basic = {
      username = "svc-user"
      password = "svc-pass"
    }
  })
}
`, name, description, allowlist)
}
