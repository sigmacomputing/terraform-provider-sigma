package connections_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccConnectionPathGrant_basic uses an existing connection (via env var)
// because resolving a path UUID requires the connection to have been
// catalog-introspected by Sigma — which doesn't happen synchronously
// during the create call inside this same test.
func TestAccConnectionPathGrant_basic(t *testing.T) {
	connID := pathEnv(t, "SIGMA_TEST_CONNECTION_ID")
	path := pathEnv(t, "SIGMA_TEST_CONNECTION_PATH") // e.g. "SAMPLE_DB/PUBLIC"
	teamName := acctest.AccRandName("team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionPathGrantConfig(connID, path, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_connection_path_grant.test", "permission", "usage"),
					resource.TestCheckResourceAttrPair("sigma_connection_path_grant.test", "team_id", "sigma_team.parent", "id"),
					resource.TestCheckResourceAttrSet("sigma_connection_path_grant.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_connection_path_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccConnectionPathGrantConfig(connID, path, teamName string) string {
	return fmt.Sprintf(`
data "sigma_connection_path" "p" {
  connection_id = %q
  path          = %q
}

resource "sigma_team" "parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_connection_path_grant" "test" {
  connection_path_id = data.sigma_connection_path.p.connection_path_id
  team_id            = sigma_team.parent.id
  permission         = "usage"
}
`, connID, path, teamName)
}
