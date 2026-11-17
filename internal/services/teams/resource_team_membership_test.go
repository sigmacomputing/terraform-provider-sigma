package teams_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccTeamMembership_basic. Both attrs are RequiresReplace; no update step.
func TestAccTeamMembership_basic(t *testing.T) {
	teamName := acctest.AccRandName("team")
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTeamMembershipConfig(teamName, email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("sigma_team_membership.test", "team_id", "sigma_team.parent", "id"),
					resource.TestCheckResourceAttrPair("sigma_team_membership.test", "member_id", "sigma_member.parent", "id"),
					resource.TestCheckResourceAttrSet("sigma_team_membership.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_team_membership.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccTeamMembershipConfig(teamName, email string) string {
	return fmt.Sprintf(`
resource "sigma_team" "parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_member" "parent" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "TeamMember"
  member_type = "view"
}

resource "sigma_team_membership" "test" {
  team_id   = sigma_team.parent.id
  member_id = sigma_member.parent.id
}
`, teamName, email)
}
