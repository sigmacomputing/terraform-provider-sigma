package tags_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccTagWorkbooksDataSource_basic verifies the data source returns
// without error for a brand-new tag (no workbooks tagged yet — list may be empty).
func TestAccTagWorkbooksDataSource_basic(t *testing.T) {
	name := acctest.AccRandName("tag")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_tag" "parent" {
  name  = %q
  color = "violet"
}

data "sigma_tag_workbooks" "test" {
  tag_id = sigma_tag.parent.id
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.sigma_tag_workbooks.test", "tag_id", "sigma_tag.parent", "id"),
					resource.TestCheckResourceAttrSet("data.sigma_tag_workbooks.test", "workbooks.#"),
				),
			},
		},
	})
}
