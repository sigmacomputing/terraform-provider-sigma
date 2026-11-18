package deploymentpolicies_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccDeploymentPolicyTenantsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_deployment_policy_tenants" "all" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.sigma_deployment_policy_tenants.all", "tenants.#"),
			},
		},
	})
}
