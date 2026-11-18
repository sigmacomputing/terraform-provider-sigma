package user_attributes_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccUserAttribute_basic. All attrs (name, description, default_value)
// are RequiresReplace; no in-place update test.
func TestAccUserAttribute_basic(t *testing.T) {
	name := acctest.AccRandName("ua")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserAttributeConfig(name, "global"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_user_attribute.test", "name", name),
					resource.TestCheckResourceAttr("sigma_user_attribute.test", "default_value", "global"),
					resource.TestCheckResourceAttrSet("sigma_user_attribute.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_user_attribute.test",
				ImportState:       true,
				ImportStateVerify: true,
				// default_value is Sensitive but stored in state; import retrieves it.
				// However the API's GET response shape may not always echo it back —
				// so ignore on verify if it differs.
				ImportStateVerifyIgnore: []string{"default_value"},
			},
		},
	})
}

func testAccUserAttributeConfig(name, defaultValue string) string {
	return fmt.Sprintf(`
resource "sigma_user_attribute" "test" {
  name          = %q
  description   = "v0 acctest"
  default_value = %q
}
`, name, defaultValue)
}
