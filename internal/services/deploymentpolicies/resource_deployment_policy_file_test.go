package deploymentpolicies_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Gated on SIGMA_TEST_INODE_ID — needs a real inode ID in the test org.
func TestAccDeploymentPolicyFile_basic(t *testing.T) {
	inodeID := os.Getenv("SIGMA_TEST_INODE_ID")
	if inodeID == "" {
		t.Skip("SIGMA_TEST_INODE_ID not set; skipping deployment-policy-file test (needs a real file/workbook inode ID)")
	}
	name := acctest.AccRandName("deploy-policy-file")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeploymentPolicyFileConfig(name, inodeID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("sigma_deployment_policy_file.test", "deployment_policy_id", "sigma_deployment_policy.test", "id"),
					resource.TestCheckResourceAttr("sigma_deployment_policy_file.test", "inode_id", inodeID),
					resource.TestCheckResourceAttrSet("sigma_deployment_policy_file.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_deployment_policy_file.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDeploymentPolicyFileConfig(name, inodeID string) string {
	return fmt.Sprintf(`
resource "sigma_deployment_policy" "test" {
  name = %q
}

resource "sigma_deployment_policy_file" "test" {
  deployment_policy_id = sigma_deployment_policy.test.id
  inode_id              = %q
}
`, name, inodeID)
}
