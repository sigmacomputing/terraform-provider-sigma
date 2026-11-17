package e2e_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccRelationships_identityContentGraph applies a single composed config that
// wires the core cross-resource graph and asserts every relationship resolves via
// TestCheckResourceAttrPair (the linked attribute equals the referenced resource's
// id). This is the committed successor to the static tmp/stress/v0_matrix.tf
// snapshot: it asserts relationships rather than merely applying them.
//
// Scope: the fixture-free identity / workspace / content graph, so it runs on the
// sandbox without external IDs or warehouse credentials. Relationships that need
// live infra (connection grants, workbook-data-source grants/version-tags) are
// covered by their own gated per-resource tests, and the connector→credential
// (auth_id) relationship is asserted in resource_api_connector_test.go.
func TestAccRelationships_identityContentGraph(t *testing.T) {
	suffix := acctest.AccRandName("rel")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRelationshipsConfig(suffix),
				Check: resource.ComposeAggregateTestCheckFunc(
					// member ←→ team via team_membership
					resource.TestCheckResourceAttrPair(
						"sigma_team_membership.m", "team_id", "sigma_team.eng", "id"),
					resource.TestCheckResourceAttrPair(
						"sigma_team_membership.m", "member_id", "sigma_member.alice", "id"),

					// team ←→ workspace via workspace_grant
					resource.TestCheckResourceAttrPair(
						"sigma_workspace_grant.ws_eng", "workspace_id", "sigma_workspace.ws", "id"),
					resource.TestCheckResourceAttrPair(
						"sigma_workspace_grant.ws_eng", "team_id", "sigma_team.eng", "id"),

					// user_attribute ←→ team and ←→ user
					resource.TestCheckResourceAttrPair(
						"sigma_user_attribute_team_assignment.eng_region", "user_attribute_id", "sigma_user_attribute.region", "id"),
					resource.TestCheckResourceAttrPair(
						"sigma_user_attribute_team_assignment.eng_region", "team_id", "sigma_team.eng", "id"),
					resource.TestCheckResourceAttrPair(
						"sigma_user_attribute_user_assignment.alice_region", "user_attribute_id", "sigma_user_attribute.region", "id"),
					resource.TestCheckResourceAttrPair(
						"sigma_user_attribute_user_assignment.alice_region", "user_id", "sigma_member.alice", "id"),

					// folder hierarchy: child.parent_id == parent.id
					resource.TestCheckResourceAttrPair(
						"sigma_folder.child", "parent_id", "sigma_folder.parent", "id"),

					// sanity: the value assignments carry the configured values
					resource.TestCheckResourceAttr(
						"sigma_user_attribute_team_assignment.eng_region", "value", "us-east"),
					resource.TestCheckResourceAttr(
						"sigma_user_attribute_user_assignment.alice_region", "value", "eu-west"),
				),
			},
		},
	})
}

func testAccRelationshipsConfig(suffix string) string {
	return fmt.Sprintf(`
resource "sigma_member" "alice" {
  email       = "alice-%[1]s@example.com"
  first_name  = "Alice"
  last_name   = "Rel"
  member_type = "Build"
  send_invite = false
}

resource "sigma_team" "eng" {
  name       = "%[1]s-eng"
  visibility = "private"
}

resource "sigma_team_membership" "m" {
  team_id   = sigma_team.eng.id
  member_id = sigma_member.alice.id
}

resource "sigma_workspace" "ws" {
  name = "%[1]s-ws"
}

resource "sigma_workspace_grant" "ws_eng" {
  workspace_id = sigma_workspace.ws.id
  team_id      = sigma_team.eng.id
  permission   = "view"
}

resource "sigma_user_attribute" "region" {
  name          = "%[1]s-region"
  description   = "RLS region scope."
  default_value = "global"
}

resource "sigma_user_attribute_team_assignment" "eng_region" {
  user_attribute_id = sigma_user_attribute.region.id
  team_id           = sigma_team.eng.id
  value             = "us-east"
}

resource "sigma_user_attribute_user_assignment" "alice_region" {
  user_attribute_id = sigma_user_attribute.region.id
  user_id           = sigma_member.alice.id
  value             = "eu-west"
}

resource "sigma_folder" "parent" {
  name = "%[1]s-parent"
}

resource "sigma_folder" "child" {
  name      = "%[1]s-child"
  parent_id = sigma_folder.parent.id
}
`, suffix)
}
