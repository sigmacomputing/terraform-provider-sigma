package sourceswap_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccSourceSwapPolicy_basic. type is RequiresReplace; name and
// connection_swaps are mutable in-place. Uses typed connection_swaps blocks
// (no more swaps_json). Gated on SIGMA_TEST_CONNECTION_ID — needs a real
// connection ID in the test org to point from_connection_id at.
func TestAccSourceSwapPolicy_basic(t *testing.T) {
	connID := os.Getenv("SIGMA_TEST_CONNECTION_ID")
	if connID == "" {
		t.Skip("SIGMA_TEST_CONNECTION_ID not set; skipping source-swap test (needs a real connection ID)")
	}
	name := acctest.AccRandName("swap")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccSourceSwapPolicyConfig(name, connID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "name", name),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "type", "connection"),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "from_connection_id", connID),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "connection_swaps.#", "1"),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "connection_swaps.0.to_connection_id", connID),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "connection_swaps.0.path0.swap_type", "exact"),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "connection_swaps.0.path0.catalog_object", "SIGMA_SAMPLE_DATABASE"),
					resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "connection_swaps.0.path1.swap_type", "all"),
					resource.TestCheckResourceAttrSet("sigma_source_swap_policy.test", "id"),
				),
			},
			{
				Config: testAccSourceSwapPolicyConfig(name+"-renamed", connID),
				Check:  resource.TestCheckResourceAttr("sigma_source_swap_policy.test", "name", name+"-renamed"),
			},
			{
				ResourceName:      "sigma_source_swap_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSourceSwapPolicyConfig(name, connID string) string {
	return fmt.Sprintf(`
resource "sigma_source_swap_policy" "test" {
  type               = "connection"
  name               = %q
  from_connection_id = %q
  connection_swaps = [
    {
      to_connection_id = %q
      path0 = { swap_type = "exact", catalog_object = "SIGMA_SAMPLE_DATABASE" }
      path1 = { swap_type = "all" }
      path2 = { swap_type = "all" }
    },
  ]
}
`, name, connID, connID)
}
