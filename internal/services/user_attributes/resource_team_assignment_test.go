package user_attributes_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccUserAttributeTeamAssignment_basic exercises in-place value updates
// (value is Required but not RequiresReplace).
func TestAccUserAttributeTeamAssignment_basic(t *testing.T) {
	uaName := acctest.AccRandName("ua")
	teamName := acctest.AccRandName("team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUATeamAssignmentConfig(uaName, teamName, "us-east"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_user_attribute_team_assignment.test", "value", "us-east"),
					resource.TestCheckResourceAttrPair("sigma_user_attribute_team_assignment.test", "user_attribute_id", "sigma_user_attribute.parent", "id"),
					resource.TestCheckResourceAttrPair("sigma_user_attribute_team_assignment.test", "team_id", "sigma_team.parent", "id"),
					resource.TestCheckResourceAttrSet("sigma_user_attribute_team_assignment.test", "id"),
				),
			},
			{
				Config: testAccUATeamAssignmentConfig(uaName, teamName, "eu-west"),
				Check:  resource.TestCheckResourceAttr("sigma_user_attribute_team_assignment.test", "value", "eu-west"),
			},
			{
				ResourceName:            "sigma_user_attribute_team_assignment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"}, // Sensitive; not always echoed by API
			},
		},
	})
}

func testAccUATeamAssignmentConfig(uaName, teamName, value string) string {
	return fmt.Sprintf(`
resource "sigma_user_attribute" "parent" {
  name          = %q
  description   = "v0 acctest"
  default_value = "global"
}

resource "sigma_team" "parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_user_attribute_team_assignment" "test" {
  user_attribute_id = sigma_user_attribute.parent.id
  team_id           = sigma_team.parent.id
  value             = %q
}
`, uaName, teamName, value)
}
