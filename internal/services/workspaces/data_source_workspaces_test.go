package workspaces_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccWorkspacesDataSource_basic(t *testing.T) {
	name := acctest.AccRandName("ws")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_workspace" "seed" {
  name = %q
}

data "sigma_workspaces" "all" {
  depends_on = [sigma_workspace.seed]
}
`, name),
				Check: resource.TestCheckResourceAttrSet("data.sigma_workspaces.all", "workspaces.#"),
			},
		},
	})
}
