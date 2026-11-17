package allowed_ips_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccAllowedIPsDataSource_basic — listing should always succeed even
// when there are no entries; we just check the data source returns
// without error.
//
// Gated on SIGMA_TEST_ALLOWED_IPS=1 because Sigma's allowedIps endpoint
// is a private beta and returns 404 unless the org has the feature flag
// enabled. The marshal logic is correct per OAS — flip the gate when
// you're testing on a beta-enabled org.
func TestAccAllowedIPsDataSource_basic(t *testing.T) {
	if os.Getenv("SIGMA_TEST_ALLOWED_IPS") != "1" {
		t.Skip("SIGMA_TEST_ALLOWED_IPS not set; skipping (beta API requires org feature flag)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_allowed_ips" "all" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.sigma_allowed_ips.all", "entries.#"),
			},
		},
	})
}
