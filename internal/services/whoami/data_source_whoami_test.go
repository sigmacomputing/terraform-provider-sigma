package whoami_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccWhoamiDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_whoami" "me" {}`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.sigma_whoami.me", "user_id"),
					resource.TestCheckResourceAttrSet("data.sigma_whoami.me", "organization_id"),
					resource.TestCheckResourceAttrPair("data.sigma_whoami.me", "id", "data.sigma_whoami.me", "user_id"),
				),
			},
		},
	})
}
