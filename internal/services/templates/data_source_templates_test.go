package templates_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccTemplatesDataSource_basic — listing succeeds even with zero
// templates in the org.
func TestAccTemplatesDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_templates" "all" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.sigma_templates.all", "entries.#"),
			},
		},
	})
}
