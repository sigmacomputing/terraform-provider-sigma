package user_attributes_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccUserAttributeUserAssignment_basic(t *testing.T) {
	uaName := acctest.AccRandName("ua")
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUAUserAssignmentConfig(uaName, email, "alpha"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_user_attribute_user_assignment.test", "value", "alpha"),
					resource.TestCheckResourceAttrPair("sigma_user_attribute_user_assignment.test", "user_attribute_id", "sigma_user_attribute.parent", "id"),
					resource.TestCheckResourceAttrPair("sigma_user_attribute_user_assignment.test", "user_id", "sigma_member.parent", "id"),
					resource.TestCheckResourceAttrSet("sigma_user_attribute_user_assignment.test", "id"),
				),
			},
			{
				Config: testAccUAUserAssignmentConfig(uaName, email, "beta"),
				Check:  resource.TestCheckResourceAttr("sigma_user_attribute_user_assignment.test", "value", "beta"),
			},
			{
				ResourceName:            "sigma_user_attribute_user_assignment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"value"}, // Sensitive
			},
		},
	})
}

func testAccUAUserAssignmentConfig(uaName, email, value string) string {
	return fmt.Sprintf(`
resource "sigma_user_attribute" "parent" {
  name          = %q
  description   = "v0 acctest"
  default_value = "global"
}

resource "sigma_member" "parent" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "UAUser"
  member_type = "view"
}

resource "sigma_user_attribute_user_assignment" "test" {
  user_attribute_id = sigma_user_attribute.parent.id
  user_id           = sigma_member.parent.id
  value             = %q
}
`, uaName, email, value)
}
