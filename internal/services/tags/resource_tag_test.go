package tags_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccTag_basic. name and color are RequiresReplace; description is mutable.
// Update step changes description in-place.
func TestAccTag_basic(t *testing.T) {
	name := acctest.AccRandName("tag")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagConfig(name, "violet", "initial"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_tag.test", "name", name),
					resource.TestCheckResourceAttr("sigma_tag.test", "color", "violet"),
					resource.TestCheckResourceAttr("sigma_tag.test", "description", "initial"),
					resource.TestCheckResourceAttrSet("sigma_tag.test", "id"),
				),
			},
			{
				Config: testAccTagConfig(name, "violet", "updated"),
				Check:  resource.TestCheckResourceAttr("sigma_tag.test", "description", "updated"),
			},
			{
				ResourceName:            "sigma_tag.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"color"}, // LIST endpoint omits color; only GET-by-id (eventually consistent) returns it
			},
		},
	})
}

func testAccTagConfig(name, color, description string) string {
	return fmt.Sprintf(`
resource "sigma_tag" "test" {
  name        = %q
  color       = %q
  description = %q
}
`, name, color, description)
}

// TestAccTag_create_without_color asserts the provider defaults `color` to
// `cyan` when omitted on create. The Sigma API requires color on POST /v2/tags,
// so the provider supplies a default rather than forwarding the bare 400.
func TestAccTag_create_without_color(t *testing.T) {
	name := acctest.AccRandName("tag")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_tag" "test" {
  name = %q
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_tag.test", "name", name),
					resource.TestCheckResourceAttr("sigma_tag.test", "color", "cyan"),
				),
			},
		},
	})
}
