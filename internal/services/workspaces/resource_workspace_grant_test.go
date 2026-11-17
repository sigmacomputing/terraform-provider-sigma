package workspaces_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccWorkspaceGrant_basic. team_id/workspace_id/member_id are
// RequiresReplace; permission is technically mutable but the provider
// implements that path as a no-op fallback. No update step.
func TestAccWorkspaceGrant_basic(t *testing.T) {
	wsName := acctest.AccRandName("ws")
	teamName := acctest.AccRandName("team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceGrantConfig(wsName, teamName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_workspace_grant.test", "permission", "view"),
					resource.TestCheckResourceAttrPair("sigma_workspace_grant.test", "workspace_id", "sigma_workspace.parent", "id"),
					resource.TestCheckResourceAttrPair("sigma_workspace_grant.test", "team_id", "sigma_team.parent", "id"),
					resource.TestCheckResourceAttrSet("sigma_workspace_grant.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_workspace_grant.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccWorkspaceGrant_multiGrant reproduces issue #14: several workspace
// grants created in a SINGLE apply. The provider discovers each new grantId by
// re-listing an eventually-consistent grant list, so the later concurrent
// grants used to fail their create read-back and get orphaned. This asserts all
// grants land in state and that a follow-up plan is a clean no-op.
func TestAccWorkspaceGrant_multiGrant(t *testing.T) {
	wsName := acctest.AccRandName("ws")
	teamPrefix := acctest.AccRandName("team")
	const n = 7
	checks := []resource.TestCheckFunc{}
	for i := 0; i < n; i++ {
		checks = append(checks,
			resource.TestCheckResourceAttrSet(fmt.Sprintf("sigma_workspace_grant.g.%d", i), "id"),
			resource.TestCheckResourceAttr(fmt.Sprintf("sigma_workspace_grant.g.%d", i), "permission", "organize"),
		)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceGrantMultiConfig(wsName, teamPrefix, n),
				Check:  resource.ComposeAggregateTestCheckFunc(checks...),
			},
			{
				// Re-apply must converge to a no-op (no orphan re-POST loop).
				Config:   testAccWorkspaceGrantMultiConfig(wsName, teamPrefix, n),
				PlanOnly: true,
			},
		},
	})
}

// Uses count (integer indices) rather than for_each — the plugin-testing state
// checks don't support string-keyed (for_each) resource indices.
func testAccWorkspaceGrantMultiConfig(wsName, teamPrefix string, n int) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "parent" {
  name = %q
}

resource "sigma_team" "t" {
  count      = %d
  name       = "%s-${count.index}"
  visibility = "private"
}

resource "sigma_workspace_grant" "g" {
  count        = %d
  workspace_id = sigma_workspace.parent.id
  team_id      = sigma_team.t[count.index].id
  permission   = "organize"
}
`, wsName, n, teamPrefix, n)
}

func testAccWorkspaceGrantConfig(wsName, teamName string) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "parent" {
  name = %q
}

resource "sigma_team" "parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_workspace_grant" "test" {
  workspace_id = sigma_workspace.parent.id
  team_id      = sigma_team.parent.id
  permission   = "view"
}
`, wsName, teamName)
}
