package e2e_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

// This file generalises the canonical drift pattern in drift_test.go beyond
// sigma_team, to the highest-value resources across the identity / workspace /
// content graph. Each resource gets:
//   - a *Disappears test: out-of-band DELETE -> refresh+plan must be non-empty.
//   - a *FieldMutation test (where the resource has a mutable, re-read field):
//     out-of-band PATCH -> refresh+plan must be non-empty.
//
// Class-B (list-based Read) resources (sigma_*_membership, grants)
// exercise the "id absent from the list -> RemoveResource" path, distinct from
// the GET-by-id 404 path the team test covers.

// ---------- sigma_member (Class A, GET-by-id) ----------

func driftMemberConfig(email string) string {
	return fmt.Sprintf(`
resource "sigma_member" "drift" {
  email       = %q
  first_name  = "Drift"
  last_name   = "AccTest"
  member_type = "view"
}
`, email)
}

func TestAccDrift_memberDisappears(t *testing.T) {
	email := acctest.AccRandName("drift-member") + "@example.com"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftMemberConfig(email),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_member.drift", "id"),
				deleteOutOfBandByID(t, "sigma_member.drift", "/v2/members/%s"),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccDrift_memberFieldMutation(t *testing.T) {
	email := acctest.AccRandName("drift-member-mut") + "@example.com"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftMemberConfig(email),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_member.drift", "id"),
				mutateOutOfBandByID(t, "sigma_member.drift", "PATCH", "/v2/members/%s",
					map[string]any{"firstName": "DriftedExternally"}),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

// ---------- sigma_workspace (Class A, GET-by-id) ----------

func driftWorkspaceConfig(name string) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "drift" {
  name = %q
}
`, name)
}

func TestAccDrift_workspaceDisappears(t *testing.T) {
	name := acctest.AccRandName("drift-ws")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftWorkspaceConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_workspace.drift", "id"),
				deleteOutOfBandByID(t, "sigma_workspace.drift", "/v2/workspaces/%s"),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccDrift_workspaceFieldMutation(t *testing.T) {
	name := acctest.AccRandName("drift-ws-mut")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftWorkspaceConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_workspace.drift", "id"),
				mutateOutOfBandByID(t, "sigma_workspace.drift", "PATCH", "/v2/workspaces/%s",
					map[string]any{"name": name + "-drifted"}),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

// ---------- sigma_folder (Class A, GET-by-id, /v2/files) ----------

func driftFolderConfig(wsName, folderName string) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "drift_parent" {
  name = %q
}

resource "sigma_folder" "drift" {
  name      = %q
  parent_id = sigma_workspace.drift_parent.id
}
`, wsName, folderName)
}

func TestAccDrift_folderDisappears(t *testing.T) {
	wsName := acctest.AccRandName("drift-folder-ws")
	folderName := acctest.AccRandName("drift-folder")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftFolderConfig(wsName, folderName),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_folder.drift", "id"),
				deleteOutOfBandByID(t, "sigma_folder.drift", "/v2/files/%s"),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

func TestAccDrift_folderFieldMutation(t *testing.T) {
	wsName := acctest.AccRandName("drift-folder-mut-ws")
	folderName := acctest.AccRandName("drift-folder-mut")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftFolderConfig(wsName, folderName),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_folder.drift", "id"),
				mutateOutOfBandByID(t, "sigma_folder.drift", "PATCH", "/v2/files/%s",
					map[string]any{"name": folderName + "-drifted"}),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

// ---------- sigma_user_attribute (Class A; replace-only, no PATCH) ----------

func driftUserAttributeConfig(name string) string {
	return fmt.Sprintf(`
resource "sigma_user_attribute" "drift" {
  name          = %q
  description   = "drift baseline"
  default_value = "baseline"
}
`, name)
}

func TestAccDrift_userAttributeDisappears(t *testing.T) {
	name := acctest.AccRandName("drift-ua")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftUserAttributeConfig(name),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_user_attribute.drift", "id"),
				deleteOutOfBandByID(t, "sigma_user_attribute.drift", "/v2/user-attributes/%s"),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

// ---------- sigma_team_membership (Class B/C, composite ID) ----------

func driftTeamMembershipConfig(teamName, email string) string {
	return fmt.Sprintf(`
resource "sigma_team" "drift_parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_member" "drift_parent" {
  email       = %q
  first_name  = "Drift"
  last_name   = "Membership"
  member_type = "view"
}

resource "sigma_team_membership" "drift" {
  team_id   = sigma_team.drift_parent.id
  member_id = sigma_member.drift_parent.id
}
`, teamName, email)
}

// removeTeamMembershipOutOfBand removes the member from the team via the API,
// bypassing Terraform, to assert the composite-ID membership Read surfaces the
// "absent from list" path as drift.
func removeTeamMembershipOutOfBand(t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		teamID, err := attrFromState(s, resourceName, "team_id")
		if err != nil {
			return err
		}
		memberID, err := attrFromState(s, resourceName, "member_id")
		if err != nil {
			return err
		}
		body := map[string]any{"remove": []string{memberID}}
		url := "/v2/teams/" + client.EscapeID(teamID) + "/members"
		if err := acctest.NewClient(t).Do(context.Background(), "PATCH", url, body, nil); err != nil {
			return fmt.Errorf("out-of-band membership removal failed: %w", err)
		}
		return nil
	}
}

func TestAccDrift_teamMembershipDisappears(t *testing.T) {
	teamName := acctest.AccRandName("drift-tm-team")
	email := acctest.AccRandName("drift-tm") + "@example.com"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftTeamMembershipConfig(teamName, email),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_team_membership.drift", "id"),
				removeTeamMembershipOutOfBand(t, "sigma_team_membership.drift"),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

// ---------- sigma_workspace_grant: drift + the permission-update regression ----------

func driftWorkspaceGrantConfig(wsName, teamName, permission string) string {
	return fmt.Sprintf(`
resource "sigma_workspace" "drift_parent" {
  name = %q
}

resource "sigma_team" "drift_parent" {
  name       = %q
  visibility = "private"
}

resource "sigma_workspace_grant" "drift" {
  workspace_id = sigma_workspace.drift_parent.id
  team_id      = sigma_team.drift_parent.id
  permission   = %q
}
`, wsName, teamName, permission)
}

// deleteWorkspaceGrantOutOfBand deletes the grant via the API to assert the
// composite-ID grant Read surfaces deletion (the "absent from list" path).
func deleteWorkspaceGrantOutOfBand(t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		wsID, err := attrFromState(s, resourceName, "workspace_id")
		if err != nil {
			return err
		}
		grantID, err := attrFromState(s, resourceName, "grant_id")
		if err != nil {
			return err
		}
		url := "/v2/workspaces/" + client.EscapeID(wsID) + "/grants/" + client.EscapeID(grantID)
		if err := acctest.NewClient(t).Do(context.Background(), "DELETE", url, nil, nil); err != nil && !client.IsNotFound(err) {
			return fmt.Errorf("out-of-band grant delete failed: %w", err)
		}
		return nil
	}
}

func TestAccDrift_workspaceGrantDisappears(t *testing.T) {
	wsName := acctest.AccRandName("drift-wsg-ws")
	teamName := acctest.AccRandName("drift-wsg-team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: driftWorkspaceGrantConfig(wsName, teamName, "view"),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("sigma_workspace_grant.drift", "grant_id"),
				deleteWorkspaceGrantOutOfBand(t, "sigma_workspace_grant.drift"),
			),
			ExpectNonEmptyPlan: true,
		}},
	})
}

type grantEntry struct {
	GrantID    string `json:"grantId"`
	Permission string `json:"permission"`
}

// checkServerWorkspaceGrantPermission asserts the grant's permission on the
// Sigma server matches want. This is the regression guard for the
// "grant permission Update is a silent no-op" bug: after changing permission in
// config and applying, the server-side value must actually change.
func checkServerWorkspaceGrantPermission(t *testing.T, resourceName, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		wsID, err := attrFromState(s, resourceName, "workspace_id")
		if err != nil {
			return err
		}
		grantID, err := attrFromState(s, resourceName, "grant_id")
		if err != nil {
			return err
		}
		all, err := client.ListAllPaged[grantEntry](context.Background(), acctest.NewClient(t),
			"/v2/workspaces/"+client.EscapeID(wsID)+"/grants", nil)
		if err != nil {
			return fmt.Errorf("listing grants failed: %w", err)
		}
		for _, g := range all {
			if g.GrantID == grantID {
				if g.Permission != want {
					return fmt.Errorf("server grant permission = %q, want %q (a config-driven permission change did not reach the API)", g.Permission, want)
				}
				return nil
			}
		}
		return fmt.Errorf("grant %s not found on server", grantID)
	}
}

// TestAccWorkspaceGrant_permissionUpdate is a regression test for the grant
// permission no-op bug. It creates a grant at "view", then changes the config
// to "organize". With the fix (RequiresReplace on permission) this destroys and
// recreates the grant so the server reflects "organize". Before the fix, Update
// is a no-op: the server keeps "view" and the post-apply plan is non-empty.
func TestAccWorkspaceGrant_permissionUpdate(t *testing.T) {
	wsName := acctest.AccRandName("perm-upd-ws")
	teamName := acctest.AccRandName("perm-upd-team")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: driftWorkspaceGrantConfig(wsName, teamName, "view"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_workspace_grant.drift", "permission", "view"),
					checkServerWorkspaceGrantPermission(t, "sigma_workspace_grant.drift", "view"),
				),
			},
			{
				Config: driftWorkspaceGrantConfig(wsName, teamName, "organize"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_workspace_grant.drift", "permission", "organize"),
					checkServerWorkspaceGrantPermission(t, "sigma_workspace_grant.drift", "organize"),
				),
			},
		},
	})
}
