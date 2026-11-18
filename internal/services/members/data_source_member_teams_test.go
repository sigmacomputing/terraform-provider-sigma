package members_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccMemberTeamsDataSource_basic(t *testing.T) {
	teamName := acctest.AccRandName("team")
	email := acctest.AccRandName("member") + "@example.invalid"
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

resource "sigma_member" "parent" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "MemberTeams"
  member_type = "view"
}

resource "sigma_team_membership" "link" {
  team_id   = sigma_team.parent.id
  member_id = sigma_member.parent.id
}

data "sigma_member_teams" "test" {
  member_id  = sigma_member.parent.id
  depends_on = [sigma_team_membership.link]
}
`, teamName, email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.sigma_member_teams.test", "member_id", "sigma_member.parent", "id"),
					resource.TestCheckResourceAttrSet("data.sigma_member_teams.test", "teams.#"),
				),
			},
		},
	})
}
