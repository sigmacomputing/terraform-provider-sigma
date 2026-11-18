package members_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccMembersDataSource_basic(t *testing.T) {
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_member" "seed" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "MembersDS"
  member_type = "view"
}

data "sigma_members" "all" {
  depends_on = [sigma_member.seed]
}
`, email),
				Check: resource.TestCheckResourceAttrSet("data.sigma_members.all", "members.#"),
			},
		},
	})
}
