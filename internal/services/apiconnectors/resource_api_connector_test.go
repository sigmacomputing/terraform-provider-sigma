package apiconnectors_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccAPIConnector_basic covers create -> update (description) -> import for a
// connector wired to a credential via auth_id. params_json/config_json are opaque
// write-only fields, ignored on import-verify.
func TestAccAPIConnector_basic(t *testing.T) {
	name := acctest.AccRandName("apiconn")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectorConfig(name, "initial"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_api_connector.test", "name", name),
					resource.TestCheckResourceAttr("sigma_api_connector.test", "description", "initial"),
					resource.TestCheckResourceAttrSet("sigma_api_connector.test", "id"),
					resource.TestCheckResourceAttrPair(
						"sigma_api_connector.test", "auth_id",
						"sigma_api_connector_credential.test", "id"),
				),
			},
			{
				Config: testAccConnectorConfig(name, "updated"),
				Check:  resource.TestCheckResourceAttr("sigma_api_connector.test", "description", "updated"),
			},
			{
				ResourceName:            "sigma_api_connector.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"params_json", "config_json"}, // opaque/write-only
			},
		},
	})
}

func testAccConnectorConfig(name, description string) string {
	return fmt.Sprintf(`
resource "sigma_api_connector_credential" "test" {
  name      = "%[1]s-cred"
  allowlist = ["*.example.com"]
  credential_json = jsonencode({
    authMethod = "basic"
    basic      = { username = "u", password = "p" }
  })
}

resource "sigma_api_connector" "test" {
  name        = %[1]q
  description = %[2]q
  auth_id     = sigma_api_connector_credential.test.id
  params_json = jsonencode({
    method      = "GET"
    url         = "https://api.example.com/v1/data"
    headers     = []
    pathParams  = []
    queryParams = []
    body        = ""
  })
}
`, name, description)
}
