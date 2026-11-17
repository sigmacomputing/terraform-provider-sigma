package jwks_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Creates a set, stages a second key, rotates to it, and imports both.
func TestAccJwkSet_rotation(t *testing.T) {
	cfgDays := func(days int, active bool) string {
		return fmt.Sprintf(`
resource "sigma_jwk_set" "test" {
  valid_for_days = %d
}

resource "sigma_jwk_set_key" "next" {
  jwk_set_id     = sigma_jwk_set.test.id
  valid_for_days = 30
  active         = %t
}
`, days, active)
	}
	cfg := func(active bool) string { return cfgDays(30, active) }
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg(false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("sigma_jwk_set.test", "active_key_id"),
					resource.TestCheckResourceAttrSet("sigma_jwk_set.test", "public_key"),
					resource.TestCheckResourceAttr("sigma_jwk_set_key.next", "active", "false"),
					resource.TestCheckResourceAttrSet("sigma_jwk_set_key.next", "public_key"),
				),
			},
			{
				Config: cfg(true),
				Check:  resource.TestCheckResourceAttr("sigma_jwk_set_key.next", "active", "true"),
			},
			{
				// Refresh: the set now reports the rotated key as active.
				Config: cfg(true),
				Check: resource.TestCheckResourceAttrPair("sigma_jwk_set.test", "active_key_id",
					"sigma_jwk_set_key.next", "key_id"),
			},
			{
				// valid_for_days is creation-only: the update keeps the active-key attributes.
				Config: cfgDays(60, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_jwk_set.test", "valid_for_days", "60"),
					resource.TestCheckResourceAttrPair("sigma_jwk_set.test", "active_key_id",
						"sigma_jwk_set_key.next", "key_id"),
				),
			},
			{
				// No deactivate call: rejected at plan time.
				Config:      cfgDays(60, false),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Can't deactivate a JSON Web Key`),
			},
			{
				ResourceName:            "sigma_jwk_set.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"valid_for_days"},
			},
			{
				ResourceName:            "sigma_jwk_set_key.next",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"valid_for_days"},
			},
		},
	})
}
