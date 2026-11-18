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

// TestAccDrift_teamDisappears asserts the provider DETECTS out-of-band deletion.
// After the team is deleted directly via the API (bypassing Terraform), the next
// refresh+plan must be non-empty — i.e. the provider's Read removes it from state
// and Terraform proposes to recreate it. This is the canonical drift test.
func TestAccDrift_teamDisappears(t *testing.T) {
	name := acctest.AccRandName("drift-del")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDriftTeamConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sigma_team.drift", "id"),
					deleteTeamOutOfBand(t, "sigma_team.drift"),
				),
				ExpectNonEmptyPlan: true, // deletion must surface as drift
			},
		},
	})
}

// TestAccDrift_teamFieldMutation asserts the provider DETECTS an out-of-band field
// change. After the team's name is changed directly via the API, refresh+plan must
// be non-empty — the provider reads the drifted value and Terraform proposes to
// revert it to the configured value.
func TestAccDrift_teamFieldMutation(t *testing.T) {
	name := acctest.AccRandName("drift-mut")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDriftTeamConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sigma_team.drift", "id"),
					renameTeamOutOfBand(t, "sigma_team.drift", name+"-drifted"),
				),
				ExpectNonEmptyPlan: true, // out-of-band rename must surface as drift
			},
		},
	})
}

func testAccDriftTeamConfig(name string) string {
	return fmt.Sprintf(`
resource "sigma_team" "drift" {
  name       = %q
  visibility = "private"
}
`, name)
}

func deleteTeamOutOfBand(t *testing.T, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := idFromState(s, resourceName)
		if err != nil {
			return err
		}
		if err := acctest.NewClient(t).Do(context.Background(), "DELETE", "/v2/teams/"+client.EscapeID(id), nil, nil); err != nil {
			return fmt.Errorf("out-of-band delete failed: %w", err)
		}
		return nil
	}
}

func renameTeamOutOfBand(t *testing.T, resourceName, newName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := idFromState(s, resourceName)
		if err != nil {
			return err
		}
		body := map[string]any{"name": newName}
		if err := acctest.NewClient(t).Do(context.Background(), "PATCH", "/v2/teams/"+client.EscapeID(id), body, nil); err != nil {
			return fmt.Errorf("out-of-band rename failed: %w", err)
		}
		return nil
	}
}

func idFromState(s *terraform.State, resourceName string) (string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", resourceName)
	}
	return rs.Primary.Attributes["id"], nil
}
