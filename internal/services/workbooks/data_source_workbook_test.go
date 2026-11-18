package workbooks_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccWorkbookDataSource_basic. Gated on SIGMA_TEST_WORKBOOK_ID — fresh
// workbooks created via API don't have latest_version populated until saved
// in the UI, so we use a fixture.
func TestAccWorkbookDataSource_basic(t *testing.T) {
	workbookID := os.Getenv("SIGMA_TEST_WORKBOOK_ID")
	if workbookID == "" {
		t.Skip("SIGMA_TEST_WORKBOOK_ID not set; skipping (needs a published-workbook fixture)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "sigma_workbook" "test" {
  id = %q
}
`, workbookID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.sigma_workbook.test", "id", workbookID),
					resource.TestCheckResourceAttrSet("data.sigma_workbook.test", "name"),
					resource.TestCheckResourceAttrSet("data.sigma_workbook.test", "latest_version"),
				),
			},
		},
	})
}
