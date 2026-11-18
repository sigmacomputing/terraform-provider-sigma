package teams_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccTeam_basic(t *testing.T) {
	name := acctest.AccRandName("team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamConfig(name, "v0 team"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_team.test", "name", name),
					resource.TestCheckResourceAttr("sigma_team.test", "description", "v0 team"),
					resource.TestCheckResourceAttr("sigma_team.test", "visibility", "private"),
					resource.TestCheckResourceAttrSet("sigma_team.test", "id"),
				),
			},
			{
				Config: testAccTeamConfig(name, "v0 team updated"),
				Check:  resource.TestCheckResourceAttr("sigma_team.test", "description", "v0 team updated"),
			},
			{
				ResourceName:      "sigma_team.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccTeamConfig(name, description string) string {
	return fmt.Sprintf(`
resource "sigma_team" "test" {
  name        = %q
  description = %q
  visibility  = "private"
}
`, name, description)
}
