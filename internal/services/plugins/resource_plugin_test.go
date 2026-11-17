package plugins_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccPlugin_basic exercises create → update → import for sigma_plugin
// (Sigma's beta custom-plugins API). name/description/dev_url update in place;
// url forces replacement.
func TestAccPlugin_basic(t *testing.T) {
	name := acctest.AccRandName("plugin")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccPluginConfig(name, "https://plugins.example.com/v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_plugin.test", "name", name),
					resource.TestCheckResourceAttr("sigma_plugin.test", "url", "https://plugins.example.com/v1"),
					resource.TestCheckResourceAttr("sigma_plugin.test", "type", "element"),
					resource.TestCheckResourceAttrSet("sigma_plugin.test", "id"),
					resource.TestCheckResourceAttrSet("sigma_plugin.test", "dev_url"),
				),
			},
			{
				Config: testAccPluginConfig(name+"-renamed", "https://plugins.example.com/v1"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("sigma_plugin.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckResourceAttr("sigma_plugin.test", "name", name+"-renamed"),
			},
			{
				Config: testAccPluginConfig(name+"-renamed", "https://plugins.example.com/v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("sigma_plugin.test", plancheck.ResourceActionReplace),
				}},
				Check: resource.TestCheckResourceAttr("sigma_plugin.test", "url", "https://plugins.example.com/v2"),
			},
			{
				ResourceName:      "sigma_plugin.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPluginConfig(name, url string) string {
	return fmt.Sprintf(`
resource "sigma_plugin" "test" {
  name        = %q
  description = "acc test plugin"
  url         = %q
}
`, name, url)
}
