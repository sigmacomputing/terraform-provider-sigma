// Package acctest is a shared harness for acceptance tests across the
// internal/services/* packages. It provides a provider factory wired to the
// real *internal/provider*.SigmaProvider plus a PreCheck that gates tests on
// the presence of Sigma sandbox credentials.
package acctest

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/provider"
)

// ProtoV6ProviderFactories returns a fresh in-process provider for each test.
// Use as the `ProtoV6ProviderFactories` field of `resource.TestCase`.
var ProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"sigma": providerserver.NewProtocol6WithError(provider.New("test")()),
}

// PreCheck fails the test early if mandatory env vars are missing.
func PreCheck(t *testing.T) {
	t.Helper()
	for _, v := range []string{"SIGMA_CLIENT_ID", "SIGMA_CLIENT_SECRET"} {
		if os.Getenv(v) == "" {
			t.Fatalf("%s must be set for acceptance tests", v)
		}
	}
}

// NewClient builds a *client.Client from the acceptance-test env vars
// (SIGMA_BASE_URL / SIGMA_CLIENT_ID / SIGMA_CLIENT_SECRET). Drift tests use it to
// mutate or delete resources OUT OF BAND (bypassing Terraform) so they can assert
// the provider's Read path detects the change. Call only after PreCheck.
func NewClient(t *testing.T) *client.Client {
	t.Helper()
	base := os.Getenv("SIGMA_BASE_URL")
	if base == "" {
		base = client.DefaultBaseURL
	}
	c, err := client.New(context.Background(), client.Config{
		BaseURL:      base,
		ClientID:     os.Getenv("SIGMA_CLIENT_ID"),
		ClientSecret: os.Getenv("SIGMA_CLIENT_SECRET"),
		UserAgent:    "terraform-provider-sigma-acctest",
	})
	if err != nil {
		t.Fatalf("acctest.NewClient: %v", err)
	}
	return c
}

// accRandCounter increments on every AccRandName call; combined with
// process-startup nanosecond and a 32-bit random tag, it makes collisions
// across parallel package-level test processes effectively impossible
// (members survive `terraform destroy` as archived records, so a stale
// "already exists" from a previous run is the actual failure mode).
var accRandCounter atomic.Uint64

// AccRandName returns a unique resource name suitable for acceptance tests.
// Format: "tf-acc-<prefix>-<nanosecond>-<counter>-<uint32>". The combination
// of monotonic time + per-process counter + crypto/rand byte makes collisions
// effectively impossible across parallel test runs and re-runs of the same
// suite.
func AccRandName(prefix string) string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	tag := binary.BigEndian.Uint32(b[:])
	n := accRandCounter.Add(1)
	return fmt.Sprintf("tf-acc-%s-%d-%d-%d", prefix, time.Now().UnixNano(), n, tag)
}
