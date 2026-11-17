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

// attrFromState returns a single attribute value from a resource in the
// Terraform state, for building out-of-band API calls against composite-ID
// resources (which store their component IDs as separate attributes).
func attrFromState(s *terraform.State, resourceName, attr string) (string, error) {
	rs, ok := s.RootModule().Resources[resourceName]
	if !ok {
		return "", fmt.Errorf("resource %s not found in state", resourceName)
	}
	v, ok := rs.Primary.Attributes[attr]
	if !ok {
		return "", fmt.Errorf("attribute %q not found on %s", attr, resourceName)
	}
	return v, nil
}

// deleteOutOfBandByID deletes a resource directly via the API, bypassing
// Terraform, so a test can assert the provider's Read path surfaces the
// deletion as drift. urlTmpl must contain a single %s placeholder for the
// resource's `id` attribute, e.g. "/v2/workspaces/%s".
func deleteOutOfBandByID(t *testing.T, resourceName, urlTmpl string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := idFromState(s, resourceName)
		if err != nil {
			return err
		}
		url := fmt.Sprintf(urlTmpl, client.EscapeID(id))
		if err := acctest.NewClient(t).Do(context.Background(), "DELETE", url, nil, nil); err != nil && !client.IsNotFound(err) {
			return fmt.Errorf("out-of-band delete %s failed: %w", url, err)
		}
		return nil
	}
}

// mutateOutOfBandByID changes a resource directly via the API, bypassing
// Terraform, so a test can assert the provider's Read path surfaces the field
// mutation as drift. urlTmpl must contain a single %s placeholder for `id`.
func mutateOutOfBandByID(t *testing.T, resourceName, method, urlTmpl string, body map[string]any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := idFromState(s, resourceName)
		if err != nil {
			return err
		}
		url := fmt.Sprintf(urlTmpl, client.EscapeID(id))
		if err := acctest.NewClient(t).Do(context.Background(), method, url, body, nil); err != nil {
			return fmt.Errorf("out-of-band %s %s failed: %w", method, url, err)
		}
		return nil
	}
}
