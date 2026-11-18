package deploymentpolicies_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// version_tag_id/source_swap_policies are untested here — they need live fixtures.
func TestAccDeploymentPolicy_basic(t *testing.T) {
	name := acctest.AccRandName("deploy-policy")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeploymentPolicyConfig(name, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_deployment_policy.test", "name", name),
					resource.TestCheckResourceAttr("sigma_deployment_policy.test", "copy_input_table_data", "false"),
					resource.TestCheckResourceAttrSet("sigma_deployment_policy.test", "id"),
				),
			},
			{
				Config: testAccDeploymentPolicyConfig(name+"-renamed", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_deployment_policy.test", "name", name+"-renamed"),
					resource.TestCheckResourceAttr("sigma_deployment_policy.test", "copy_input_table_data", "true"),
				),
			},
			{
				ResourceName:      "sigma_deployment_policy.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDeploymentPolicyConfig(name string, copyInputTableData bool) string {
	return fmt.Sprintf(`
resource "sigma_deployment_policy" "test" {
  name                  = %q
  copy_input_table_data = %t
}
`, name, copyInputTableData)
}
