package teams_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccTeamsDataSource_basic(t *testing.T) {
	name := acctest.AccRandName("team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_team" "seed" {
  name       = %q
  visibility = "private"
}

data "sigma_teams" "all" {
  depends_on = [sigma_team.seed]
}
`, name),
				Check: resource.TestCheckResourceAttrSet("data.sigma_teams.all", "teams.#"),
			},
		},
	})
}
