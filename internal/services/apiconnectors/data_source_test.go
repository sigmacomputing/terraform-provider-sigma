package apiconnectors_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// TestAccAPIConnectorsDataSource_basic creates a connector (+credential) and
// asserts it appears in the list data source.
func TestAccAPIConnectorsDataSource_basic(t *testing.T) {
	name := acctest.AccRandName("apiconn-ds")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConnectorDataSourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.sigma_api_connectors.all", "connectors.#"),
					checkListContainsName("data.sigma_api_connectors.all", "connectors", name),
				),
			},
		},
	})
}

// TestAccAPIConnectorCredentialsDataSource_basic creates a credential and asserts
// it appears in the list. The list schema has no `credential_json`/secret field
// at all, so secrets cannot leak through it — this test also asserts that the
// expected non-secret attributes are the only ones present per entry.
func TestAccAPIConnectorCredentialsDataSource_basic(t *testing.T) {
	name := acctest.AccRandName("apicred-ds")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialDataSourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.sigma_api_connector_credentials.all", "credentials.#"),
					checkListContainsName("data.sigma_api_connector_credentials.all", "credentials", name),
					// Defense-in-depth: the entry must NOT carry any secret/credential field.
					checkListEntryHasNoSecret("data.sigma_api_connector_credentials.all", "credentials", name),
				),
			},
		},
	})
}

func testAccConnectorDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "sigma_api_connector_credential" "test" {
  name      = "%[1]s-cred"
  allowlist = ["*.example.com"]
  credential_json = jsonencode({
    authMethod = "basic"
    basic      = { username = "u", password = "p" }
  })
}

resource "sigma_api_connector" "test" {
  name        = %[1]q
  auth_id     = sigma_api_connector_credential.test.id
  params_json = jsonencode({
    method = "GET", url = "https://api.example.com/v1/data"
    headers = [], pathParams = [], queryParams = [], body = ""
  })
}

data "sigma_api_connectors" "all" {
  depends_on = [sigma_api_connector.test]
}
`, name)
}

func testAccCredentialDataSourceConfig(name string) string {
	return fmt.Sprintf(`
resource "sigma_api_connector_credential" "test" {
  name      = %[1]q
  allowlist = ["*.example.com"]
  credential_json = jsonencode({
    authMethod = "basic"
    basic      = { username = "u", password = "p" }
  })
}

data "sigma_api_connector_credentials" "all" {
  depends_on = [sigma_api_connector_credential.test]
}
`, name)
}

// checkListContainsName asserts that a list-nested attribute (e.g. "connectors")
// on a data source contains an element whose `name` equals want.
func checkListContainsName(resourceName, listAttr, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attrs, err := listAttrs(s, resourceName)
		if err != nil {
			return err
		}
		count, _ := strconv.Atoi(attrs[listAttr+".#"])
		for i := 0; i < count; i++ {
			if attrs[fmt.Sprintf("%s.%d.name", listAttr, i)] == want {
				return nil
			}
		}
		return fmt.Errorf("%s.%s did not contain an entry named %q (count=%d)", resourceName, listAttr, want, count)
	}
}

// checkListEntryHasNoSecret asserts the matching entry exposes no credential/secret key.
func checkListEntryHasNoSecret(resourceName, listAttr, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		attrs, err := listAttrs(s, resourceName)
		if err != nil {
			return err
		}
		count, _ := strconv.Atoi(attrs[listAttr+".#"])
		for i := 0; i < count; i++ {
			if attrs[fmt.Sprintf("%s.%d.name", listAttr, i)] != want {
				continue
			}
			prefix := fmt.Sprintf("%s.%d.", listAttr, i)
			for k := range attrs {
				if len(k) > len(prefix) && k[:len(prefix)] == prefix {
					field := k[len(prefix):]
					switch field {
					case "credential", "credential_json", "secret", "password":
						return fmt.Errorf("credentials list leaked secret field %q", field)
					}
				}
			}
			return nil
		}
		return fmt.Errorf("%s.%s had no entry named %q", resourceName, listAttr, want)
	}
}

func listAttrs(s *terraform.State, resourceName string) (map[string]string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return nil, fmt.Errorf("resource %s not found in state", resourceName)
	}
	return rs.Primary.Attributes, nil
}
