package workspaces_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccWorkspace_basic(t *testing.T) {
	name1 := acctest.AccRandName("ws")
	name2 := name1 + "-renamed"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceConfig(name1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_workspace.test", "name", name1),
					resource.TestCheckResourceAttrSet("sigma_workspace.test", "id"),
					resource.TestCheckResourceAttrSet("sigma_workspace.test", "workspace_url_id"),
				),
			},
			{
				Config: testAccWorkspaceConfig(name2),
				Check:  resource.TestCheckResourceAttr("sigma_workspace.test", "name", name2),
			},
			{
				ResourceName:      "sigma_workspace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccWorkspaceConfig(name string) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "test" {
  name = %q
}
`, name)
}
