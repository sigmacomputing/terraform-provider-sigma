package teams_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccTeamUserAttributesDataSource_basic(t *testing.T) {
	teamName := acctest.AccRandName("team")
	uaName := acctest.AccRandName("ua")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_team" "parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_user_attribute" "parent" {
  name          = %q
  description   = "v0 acctest"
  default_value = "global"
}

resource "sigma_user_attribute_team_assignment" "link" {
  user_attribute_id = sigma_user_attribute.parent.id
  team_id           = sigma_team.parent.id
  value             = "us-east"
}

data "sigma_team_user_attributes" "test" {
  team_id    = sigma_team.parent.id
  depends_on = [sigma_user_attribute_team_assignment.link]
}
`, teamName, uaName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.sigma_team_user_attributes.test", "team_id", "sigma_team.parent", "id"),
					resource.TestCheckResourceAttrSet("data.sigma_team_user_attributes.test", "user_attributes.#"),
				),
			},
		},
	})
}
