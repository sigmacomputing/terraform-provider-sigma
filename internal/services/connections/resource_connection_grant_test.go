package connections_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccConnectionGrant_basic(t *testing.T) {
	account, user, privateKey := snowflakeEnv(t)
	connName := acctest.AccRandName("conn")
	teamName := acctest.AccRandName("team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionGrantConfig(connName, teamName, account, user, privateKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_connection_grant.test", "permission", "usage"),
					resource.TestCheckResourceAttrPair("sigma_connection_grant.test", "connection_id", "sigma_connection.parent", "id"),
					resource.TestCheckResourceAttrPair("sigma_connection_grant.test", "team_id", "sigma_team.parent", "id"),
				),
			},
			{
				ResourceName:      "sigma_connection_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccConnectionGrantConfig(connName, teamName, account, user, privateKey string) string {
	return fmt.Sprintf(`
resource "sigma_connection" "parent" {
  name = %q
  details_json = jsonencode({
    type       = "snowflake"
    authType   = "key-pair"
    account    = %q
    user       = %q
    privateKey = %q
  })
}

resource "sigma_team" "parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_connection_grant" "test" {
  connection_id = sigma_connection.parent.id
  team_id       = sigma_team.parent.id
  permission    = "usage"
}
`, connName, account, user, privateKey, teamName)
}
