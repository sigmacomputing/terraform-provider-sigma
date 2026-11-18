package connections_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccConnection_basic exercises a real Snowflake connection end-to-end.
// Gated on SIGMA_TEST_SNOWFLAKE_* env vars — Sigma validates connectivity at
// create time, so we need live credentials. details_json is write-only
// (Sigma's GET strips credentials), so it's ignored on import verify.
func TestAccConnection_basic(t *testing.T) {
	account, user, privateKey := snowflakeEnv(t)
	name := acctest.AccRandName("conn")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionConfig(name, account, user, privateKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_connection.test", "name", name),
					resource.TestCheckResourceAttr("sigma_connection.test", "type", "snowflake"),
					resource.TestCheckResourceAttrSet("sigma_connection.test", "id"),
					// oauth_client_id is a Computed field (added in v0.1.1). A key-pair
					// Snowflake connection doesn't use OAuth, so it must round-trip as a
					// known empty string — this locks the field into state handling.
					resource.TestCheckResourceAttr("sigma_connection.test", "oauth_client_id", ""),
				),
			},
			{
				Config: testAccConnectionConfig(name+"-renamed", account, user, privateKey),
				Check:  resource.TestCheckResourceAttr("sigma_connection.test", "name", name+"-renamed"),
			},
			{
				ResourceName:            "sigma_connection.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"details_json"}, // write-only
			},
		},
	})
}

func testAccConnectionConfig(name, account, user, privateKey string) string {
	return fmt.Sprintf(`
resource "sigma_connection" "test" {
  name = %q
  details_json = jsonencode({
    type       = "snowflake"
    authType   = "key-pair"
    account    = %q
    user       = %q
    privateKey = %q
  })
}
`, name, account, user, privateKey)
}
