package deploymentpolicies_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Gated on SIGMA_TEST_TENANT_ORGANIZATION_ID — needs a real tenant org ID.
func TestAccDeploymentPolicyTenant_basic(t *testing.T) {
	tenantID := os.Getenv("SIGMA_TEST_TENANT_ORGANIZATION_ID")
	if tenantID == "" {
		t.Skip("SIGMA_TEST_TENANT_ORGANIZATION_ID not set; skipping deployment-policy-tenant test (needs a real tenant organization ID)")
	}
	name := acctest.AccRandName("deploy-policy-tenant")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDeploymentPolicyTenantConfig(name, tenantID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("sigma_deployment_policy_tenant.test", "deployment_policy_id", "sigma_deployment_policy.test", "id"),
					resource.TestCheckResourceAttr("sigma_deployment_policy_tenant.test", "tenant_organization_id", tenantID),
					resource.TestCheckResourceAttrSet("sigma_deployment_policy_tenant.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_deployment_policy_tenant.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDeploymentPolicyTenantConfig(name, tenantID string) string {
	return fmt.Sprintf(`
resource "sigma_deployment_policy" "test" {
  name = %q
}

resource "sigma_deployment_policy_tenant" "test" {
  deployment_policy_id   = sigma_deployment_policy.test.id
  tenant_organization_id = %q
}
`, name, tenantID)
}
