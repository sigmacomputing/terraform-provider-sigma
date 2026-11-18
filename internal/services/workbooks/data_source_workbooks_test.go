package workbooks_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccWorkbooksDataSource_basic verifies the list call succeeds and returns a
// known count. The provider no longer creates workbooks, so we don't seed one;
// the org may have zero or more workbooks (authored in the UI) and `workbooks.#`
// must be a known value either way.
func TestAccWorkbooksDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_workbooks" "all" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.sigma_workbooks.all", "workbooks.#"),
			},
		},
	})
}
