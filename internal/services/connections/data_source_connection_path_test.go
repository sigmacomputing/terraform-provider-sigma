package connections_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccConnectionPathDataSource_basic(t *testing.T) {
	connID := pathEnv(t, "SIGMA_TEST_CONNECTION_ID")
	path := pathEnv(t, "SIGMA_TEST_CONNECTION_PATH")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionPathDSConfig(connID, path),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.sigma_connection_path.test", "connection_id", connID),
					resource.TestCheckResourceAttr("data.sigma_connection_path.test", "path", path),
					resource.TestCheckResourceAttrSet("data.sigma_connection_path.test", "connection_path_id"),
				),
			},
		},
	})
}

func testAccConnectionPathDSConfig(connID, path string) string {
	return fmt.Sprintf(`
data "sigma_connection_path" "test" {
  connection_id = %q
  path          = %q
}
`, connID, path)
}
