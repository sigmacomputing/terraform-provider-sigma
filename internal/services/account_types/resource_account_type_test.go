package account_types_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Built-in "view" account type — its permission set is the seed for custom types.
const builtinViewAccountTypeID = "20140502-5fca-4000-8000-000000000004"

// TestAccAccountType_basic covers create + import. See TestAccAccountType_permissionsInPlace for updates.
func TestAccAccountType_basic(t *testing.T) {
	name := acctest.AccRandName("at")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountTypeConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("sigma_account_type.test", "name", name),
					resource.TestCheckResourceAttr("sigma_account_type.test", "description", "v0 acctest"),
					resource.TestCheckResourceAttr("sigma_account_type.test", "is_custom", "true"),
					resource.TestCheckResourceAttrSet("sigma_account_type.test", "id"),
				),
			},
			{
				ResourceName:      "sigma_account_type.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccAccountType_permissionsInPlace asserts a permissions change updates the account type
// in place (same id) while a name change replaces it (new id).
func TestAccAccountType_permissionsInPlace(t *testing.T) {
	name := acctest.AccRandName("atp")
	var firstID, secondID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAccountTypePermsConfig(name, "setsubtract(data.sigma_account_type_permissions.view.permissions, [\"comment\"])"),
				Check: resource.ComposeAggregateTestCheckFunc(
					captureID("sigma_account_type.test", &firstID),
					testCheckPermission("sigma_account_type.test", "comment", false),
				),
			},
			{
				Config: testAccAccountTypePermsConfig(name, "data.sigma_account_type_permissions.view.permissions"),
				Check: resource.ComposeAggregateTestCheckFunc(
					captureID("sigma_account_type.test", &secondID),
					testCheckPermission("sigma_account_type.test", "comment", true),
					func(*terraform.State) error {
						if firstID != secondID {
							return fmt.Errorf("permissions change replaced the account type (id %s -> %s), want in-place", firstID, secondID)
						}
						return nil
					},
				),
			},
		},
	})
}

func captureID(addr string, dst *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		*dst = rs.Primary.ID
		return nil
	}
}

// testCheckPermission asserts a permission is (or is not) in the set attribute.
func testCheckPermission(addr, perm string, want bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[addr]
		if !ok {
			return fmt.Errorf("%s not in state", addr)
		}
		found := false
		for k, v := range rs.Primary.Attributes {
			if strings.HasPrefix(k, "permissions.") && k != "permissions.#" && v == perm {
				found = true
			}
		}
		if found != want {
			return fmt.Errorf("permission %q present=%v, want %v", perm, found, want)
		}
		return nil
	}
}

func testAccAccountTypePermsConfig(name, permsExpr string) string {
	return fmt.Sprintf(`
data "sigma_account_type_permissions" "view" {
  account_type_id = %q
}

resource "sigma_account_type" "test" {
  name        = %q
  description = "permissions in-place acctest"
  permissions = %s
}
`, builtinViewAccountTypeID, name, permsExpr)
}

func testAccAccountTypeConfig(name string) string {
	return fmt.Sprintf(`
data "sigma_account_type_permissions" "view" {
  account_type_id = %q
}

resource "sigma_account_type" "test" {
  name        = %q
  description = "v0 acctest"
  permissions = data.sigma_account_type_permissions.view.permissions
}
`, builtinViewAccountTypeID, name)
}
