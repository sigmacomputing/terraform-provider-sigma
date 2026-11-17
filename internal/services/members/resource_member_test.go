package members_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccMember_basic exercises create / read / update / import / destroy
// against a real Sigma sandbox. Gated on TF_ACC=1 and the SIGMA_* env vars.
func TestAccMember_basic(t *testing.T) {
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMemberConfig(email, "Alice", "Build"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_member.test", "email", email),
					resource.TestCheckResourceAttr("sigma_member.test", "first_name", "Alice"),
					resource.TestCheckResourceAttrSet("sigma_member.test", "id"),
					resource.TestCheckResourceAttrSet("sigma_member.test", "home_folder_id"),
				),
			},
			{
				Config: testAccMemberConfig(email, "Alicia", "Build"),
				Check:  resource.TestCheckResourceAttr("sigma_member.test", "first_name", "Alicia"),
			},
			{
				ResourceName:      "sigma_member.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccMemberConfig(email, firstName, accountType string) string {
	return fmt.Sprintf(`
resource "sigma_member" "test" {
  email       = %q
  first_name  = %q
  last_name   = "AccTest"
  member_type = %q
}
`, email, firstName, accountType)
}
