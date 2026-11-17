package members_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccMemberAccountType_basic assigns, changes and imports an account type on a seeded member.
func TestAccMemberAccountType_basic(t *testing.T) {
	email := acctest.AccRandName("matype") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMemberAccountTypeConfig(email, "view"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_member_account_type.test", "account_type", "view"),
					resource.TestCheckResourceAttr("sigma_member_account_type.test", "reset_to", "Build"),
					resource.TestCheckResourceAttrPair("sigma_member_account_type.test", "id", "sigma_member.seed", "id"),
				),
			},
			{
				Config: testAccMemberAccountTypeConfig(email, "analyze"),
				Check:  resource.TestCheckResourceAttr("sigma_member_account_type.test", "account_type", "analyze"),
			},
			{
				ResourceName:      "sigma_member_account_type.test",
				ImportState:       true,
				ImportStateId:     email,
				ImportStateVerify: true,
				// reset_to isn't stored by Sigma; import uses the default.
				ImportStateVerifyIgnore: []string{"reset_to"},
			},
		},
	})
}

// TestAccMemberAccountType_missingMember asserts the resource refuses to create a member.
func TestAccMemberAccountType_missingMember(t *testing.T) {
	email := acctest.AccRandName("nobody") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_member_account_type" "test" {
  email        = %q
  account_type = "explorer"
}
`, email),
				ExpectError: regexpMemberNotFound,
			},
		},
	})
}

func testAccMemberAccountTypeConfig(email, accountType string) string {
	return fmt.Sprintf(`
resource "sigma_member" "seed" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "MemberAccountType"
  member_type = "Build"

  # The account type is owned by sigma_member_account_type, not the seed.
  lifecycle {
    ignore_changes = [member_type]
  }
}

resource "sigma_member_account_type" "test" {
  email        = sigma_member.seed.email
  account_type = %q
  reset_to     = "Build"
}
`, email, accountType)
}

var regexpMemberNotFound = regexp.MustCompile(`Member not found`)
