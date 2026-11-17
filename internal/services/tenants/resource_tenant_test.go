package tenants_test

import "testing"

// TestAccTenant_basic is a placeholder. Multitenancy is disabled on the test
// org (POST returns 400 "Multitenancy is disabled"), so the resource cannot
// be exercised end-to-end here. Set SIGMA_TEST_TENANT_PARENT=1 (or remove
// this skip) on an org with multitenancy enabled to run a real test.
func TestAccTenant_basic(t *testing.T) {
	t.Skip("multitenancy not enabled on test org — sigma_tenant requires a parent org with multitenancy provisioned")
}
