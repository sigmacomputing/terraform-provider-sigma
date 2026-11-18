package orgsettings_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

type timezoneSetting struct {
	Timezone string `json:"timezone"`
}

// Mutates live org settings, so gated behind SIGMA_TEST_ORG_SETTINGS=1; restores
// the original timezone in t.Cleanup since Delete is a no-op.
func TestAccOrganizationSettings_basic(t *testing.T) {
	if os.Getenv("SIGMA_TEST_ORG_SETTINGS") != "1" {
		t.Skip("SIGMA_TEST_ORG_SETTINGS=1 not set; skipping organization-settings test (mutates live, shared org-wide settings)")
	}
	acctest.PreCheck(t)
	c := acctest.NewClient(t)

	var original timezoneSetting
	if err := c.Do(context.Background(), "GET", "/v2/organizations/settings/timezone", nil, &original); err != nil {
		t.Fatalf("failed to capture original timezone before test: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Do(context.Background(), "PATCH", "/v2/organizations/settings/timezone", timezoneSetting{Timezone: original.Timezone}, nil); err != nil {
			t.Errorf("failed to restore original timezone %q after test: %v", original.Timezone, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationSettingsConfig("America/Los_Angeles", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_organization_settings.test", "timezone.timezone", "America/Los_Angeles"),
					resource.TestCheckResourceAttr("sigma_organization_settings.test", "bulk_copy.enabled", "false"),
					// csv_upload is unconfigured — should still populate from live state.
					resource.TestCheckResourceAttrSet("sigma_organization_settings.test", "csv_upload.enabled"),
					resource.TestCheckResourceAttr("sigma_organization_settings.test", "id", "organization"),
				),
			},
			{
				Config: testAccOrganizationSettingsConfig("America/New_York", false),
				Check:  resource.TestCheckResourceAttr("sigma_organization_settings.test", "timezone.timezone", "America/New_York"),
			},
			{
				ResourceName:      "sigma_organization_settings.test",
				ImportState:       true,
				ImportStateId:     "organization",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccOrganizationSettingsConfig(timezone string, bulkCopyEnabled bool) string {
	return `
resource "sigma_organization_settings" "test" {
  timezone = {
    timezone = "` + timezone + `"
  }
  bulk_copy = {
    enabled = ` + boolStr(bulkCopyEnabled) + `
  }
}
`
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
