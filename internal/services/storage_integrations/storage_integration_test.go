package storage_integrations_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Gated on SIGMA_TEST_S3_BUCKET / SIGMA_TEST_S3_ROLE_ARN — needs a real bucket and role.
func TestAccStorageIntegration_s3(t *testing.T) {
	bucket, role := os.Getenv("SIGMA_TEST_S3_BUCKET"), os.Getenv("SIGMA_TEST_S3_ROLE_ARN")
	if bucket == "" || role == "" {
		t.Skip("SIGMA_TEST_S3_{BUCKET,ROLE_ARN} not set; skipping storage integration test")
	}
	cfg := func(name string) string {
		return fmt.Sprintf(`
resource "sigma_storage_integration" "test" {
  provider_type        = "s3"
  name                 = %q
  bucket_name          = %q
  path_prefix          = "tf-acc/"
  storage_aws_role_arn = %q
}

data "sigma_storage_integrations" "all" {
  depends_on = [sigma_storage_integration.test]
}
`, name, bucket, role)
	}
	name := acctest.AccRandName("storage")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_storage_integration.test", "name", name),
					resource.TestCheckResourceAttrSet("sigma_storage_integration.test", "external_id"),
					resource.TestCheckResourceAttrSet("data.sigma_storage_integrations.all", "entries.#"),
				),
			},
			{
				Config: cfg(name + "-renamed"),
				Check:  resource.TestCheckResourceAttr("sigma_storage_integration.test", "name", name+"-renamed"),
			},
			{
				ResourceName:            "sigma_storage_integration.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bucket_name", "path_prefix"},
			},
		},
	})
}
