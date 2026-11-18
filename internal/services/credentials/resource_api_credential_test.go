package credentials_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccAPICredential_basic. owner_id, name, description are all
// RequiresReplace; no in-place update. client_secret is returned once
// at create and is never echoed back, so it's ignored on import verify.
func TestAccAPICredential_basic(t *testing.T) {
	credName := acctest.AccRandName("cred")
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAPICredentialConfig(credName, email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_api_credential.test", "name", credName),
					resource.TestCheckResourceAttrSet("sigma_api_credential.test", "client_id"),
					resource.TestCheckResourceAttrSet("sigma_api_credential.test", "client_secret"),
					resource.TestCheckResourceAttrPair("sigma_api_credential.test", "owner_id", "sigma_member.owner", "id"),
				),
			},
			{
				ResourceName:      "sigma_api_credential.test",
				ImportState:       true,
				ImportStateVerify: true,
				// Sigma's GET /v2/apiCredentials/{id} returns only client_id —
				// not the originally-supplied name/owner_id/scopes, and
				// (correctly) not the once-only client_secret. README's "Lifecycle
				// nuances" calls this out: imports recover client_id only.
				ImportStateVerifyIgnore: []string{"client_secret", "name", "owner_id", "scopes"},
			},
		},
	})
}

func testAccAPICredentialConfig(name, email string) string {
	return fmt.Sprintf(`
resource "sigma_member" "owner" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "Cred"
  member_type = "Build"
}

resource "sigma_api_credential" "test" {
  owner_id = sigma_member.owner.id
  name     = %q
  scopes   = ["api"]
}
`, email, name)
}
