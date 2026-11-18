package account_types_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccAccountTypePermissionsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "sigma_account_type_permissions" "view" {
  account_type_id = %q
}
`, builtinViewAccountTypeID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.sigma_account_type_permissions.view", "account_type_id", builtinViewAccountTypeID),
					resource.TestCheckResourceAttrSet("data.sigma_account_type_permissions.view", "permissions.#"),
				),
			},
		},
	})
}
