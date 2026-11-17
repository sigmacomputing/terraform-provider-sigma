package shared_templates_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccSharedTemplatesDataSource_basic — listing should succeed even with
// zero pending shares.
func TestAccSharedTemplatesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_shared_templates" "all" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.sigma_shared_templates.all", "entries.#"),
			},
		},
	})
}
