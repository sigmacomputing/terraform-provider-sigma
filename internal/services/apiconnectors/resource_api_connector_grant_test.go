package apiconnectors_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccAPIConnectorGrant_basic covers create (default `view` = Can use) -> no-op re-plan -> import.
func TestAccAPIConnectorGrant_basic(t *testing.T) {
	name := acctest.AccRandName("apiconngrant")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectorGrantConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_api_connector_grant.test", "permission", "view"),
					resource.TestCheckResourceAttrPair("sigma_api_connector_grant.test", "api_connector_id", "sigma_api_connector.test", "id"),
					resource.TestCheckResourceAttrPair("sigma_api_connector_grant.test", "team_id", "sigma_team.test", "id"),
					resource.TestCheckResourceAttrSet("sigma_api_connector_grant.test", "grant_id"),
				),
			},
			{
				Config:   testAccConnectorGrantConfig(name),
				PlanOnly: true,
			},
			{
				ResourceName:      "sigma_api_connector_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccConnectorGrantConfig(name string) string {
	return testAccConnectorConfig(name, "grant test") + fmt.Sprintf(`
resource "sigma_team" "test" {
  name       = %q
  visibility = "private"
}

resource "sigma_api_connector_grant" "test" {
  api_connector_id = sigma_api_connector.test.id
  team_id          = sigma_team.test.id
}
`, name+"-team")
}
