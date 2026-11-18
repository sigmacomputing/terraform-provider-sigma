package smtp_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

// Gated on SIGMA_TEST_SMTP_HOST/PORT/USERNAME/PASSWORD — needs a real SMTP server.
func TestAccSMTPSettings_basic(t *testing.T) {
	host, port := os.Getenv("SIGMA_TEST_SMTP_HOST"), os.Getenv("SIGMA_TEST_SMTP_PORT")
	user, pass := os.Getenv("SIGMA_TEST_SMTP_USERNAME"), os.Getenv("SIGMA_TEST_SMTP_PASSWORD")
	if host == "" || port == "" || user == "" || pass == "" {
		t.Skip("SIGMA_TEST_SMTP_{HOST,PORT,USERNAME,PASSWORD} not set; skipping SMTP settings test")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_smtp_settings" "test" {
  host         = %q
  port         = %q
  username     = %q
  password     = %q
  sender_email = "tf-acc@example.com"
}
`, host, port, user, pass),
				Check: resource.TestCheckResourceAttr("sigma_smtp_settings.test", "host", host),
			},
			{
				ResourceName:            "sigma_smtp_settings.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}
