package connections_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccConnectionTableColumnsDataSource_basic reads columns for a known
// Sigma-tracked table. Gated on SIGMA_TEST_TABLE_ID — pre-resolved table UUID.
func TestAccConnectionTableColumnsDataSource_basic(t *testing.T) {
	tableID := pathEnv(t, "SIGMA_TEST_TABLE_ID")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTableColumnsDSConfig(tableID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.sigma_connection_table_columns.test", "table_id", tableID),
					resource.TestCheckResourceAttrSet("data.sigma_connection_table_columns.test", "columns.#"),
				),
			},
		},
	})
}

func testAccTableColumnsDSConfig(tableID string) string {
	return fmt.Sprintf(`
data "sigma_connection_table_columns" "test" {
  table_id = %q
}
`, tableID)
}
