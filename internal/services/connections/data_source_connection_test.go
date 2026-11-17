package connections_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Validation + not-found paths need only Sigma auth (no warehouse), so they run
// unconditionally under TF_ACC.

func TestAccConnectionDataSource_requiresExactlyOne(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      `data "sigma_connection" "none" {}`,
				ExpectError: regexp.MustCompile(`Set exactly one`),
			},
			{
				Config: `data "sigma_connection" "both" {
  id   = "abc"
  name = "xyz"
}`,
				ExpectError: regexp.MustCompile(`Set exactly one`),
			},
		},
	})
}

func TestAccConnectionDataSource_notFound(t *testing.T) {
	name := acctest.AccRandName("no-such-conn")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(`data "sigma_connection" "missing" { name = %q }`, name),
				ExpectError: regexp.MustCompile(`No connection named`),
			},
		},
	})
}

// Seeded lookups need a live warehouse (Sigma validates connectivity at create),
// so they're gated on SIGMA_TEST_SNOWFLAKE_* like the connection resource tests.

func TestAccConnectionDataSource_byName(t *testing.T) {
	account, user, privateKey := snowflakeEnv(t)
	name := acctest.AccRandName("conn-ds")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionConfig(name, account, user, privateKey) + fmt.Sprintf(`
data "sigma_connection" "by_name" {
  name       = %q
  depends_on = [sigma_connection.test]
}
`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.sigma_connection.by_name", "id", "sigma_connection.test", "id"),
					resource.TestCheckResourceAttr("data.sigma_connection.by_name", "type", "snowflake"),
				),
			},
		},
	})
}

func TestAccConnectionDataSource_byID(t *testing.T) {
	account, user, privateKey := snowflakeEnv(t)
	name := acctest.AccRandName("conn-ds")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionConfig(name, account, user, privateKey) + `
data "sigma_connection" "by_id" {
  id = sigma_connection.test.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.sigma_connection.by_id", "name", name),
					resource.TestCheckResourceAttr("data.sigma_connection.by_id", "type", "snowflake"),
				),
			},
		},
	})
}
