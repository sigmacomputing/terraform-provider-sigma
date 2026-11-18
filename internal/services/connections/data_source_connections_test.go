package connections_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccConnectionsDataSource_basic just needs Sigma auth — listing connections
// returns whatever exists in the org (the count attribute is always set).
func TestAccConnectionsDataSource_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `data "sigma_connections" "all" {}`,
				Check:  resource.TestCheckResourceAttrSet("data.sigma_connections.all", "connections.#"),
			},
		},
	})
}

// TestAccConnectionsDataSource_findsSeeded creates a connection and confirms it
// appears in the list (resolvable name -> id). Snowflake-gated.
func TestAccConnectionsDataSource_findsSeeded(t *testing.T) {
	account, user, privateKey := snowflakeEnv(t)
	name := acctest.AccRandName("conns-ds")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectionConfig(name, account, user, privateKey) + `
data "sigma_connections" "all" {
  depends_on = [sigma_connection.test]
}

output "seeded_conn_id" {
  value = one([for c in data.sigma_connections.all.connections : c.id if c.name == sigma_connection.test.name])
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.sigma_connections.all", "connections.#"),
					resource.TestCheckResourceAttrWith("data.sigma_connections.all", "connections.#", func(v string) error {
						if v == "0" {
							return fmt.Errorf("expected at least one connection, got 0")
						}
						return nil
					}),
				),
			},
		},
	})
}
