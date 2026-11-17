package folders_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccFolder_basic(t *testing.T) {
	wsName := acctest.AccRandName("ws")
	folderName := acctest.AccRandName("folder")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFolderConfig(wsName, folderName, "initial"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_folder.test", "name", folderName),
					resource.TestCheckResourceAttr("sigma_folder.test", "description", "initial"),
					resource.TestCheckResourceAttrSet("sigma_folder.test", "id"),
					resource.TestCheckResourceAttrPair("sigma_folder.test", "parent_id", "sigma_workspace.parent", "id"),
				),
			},
			{
				Config: testAccFolderConfig(wsName, folderName, "updated"),
				Check:  resource.TestCheckResourceAttr("sigma_folder.test", "description", "updated"),
			},
			{
				ResourceName:      "sigma_folder.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccFolderConfig(wsName, folderName, description string) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "parent" {
  name = %q
}

resource "sigma_folder" "test" {
  name        = %q
  parent_id   = sigma_workspace.parent.id
  description = %q
}
`, wsName, folderName, description)
}
