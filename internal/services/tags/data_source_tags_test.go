package tags_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccTagsDataSource_basic(t *testing.T) {
	name := acctest.AccRandName("tag")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_tag" "seed" {
  name  = %q
  color = "violet"
}

data "sigma_tags" "all" {
  depends_on = [sigma_tag.seed]
}
`, name),
				Check: resource.TestCheckResourceAttrSet("data.sigma_tags.all", "tags.#"),
			},
		},
	})
}
