package members_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/acctest"
)

func TestAccMemberDataSource_byEmail(t *testing.T) {
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_member" "seed" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "MemberDSByEmail"
  member_type = "view"
}

data "sigma_member" "by_email" {
  email      = sigma_member.seed.email
  depends_on = [sigma_member.seed]
}
`, email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.sigma_member.by_email", "id", "sigma_member.seed", "id"),
					resource.TestCheckResourceAttr("data.sigma_member.by_email", "email", email),
					resource.TestCheckResourceAttr("data.sigma_member.by_email", "first_name", "AccTest"),
					resource.TestCheckResourceAttr("data.sigma_member.by_email", "last_name", "MemberDSByEmail"),
					resource.TestCheckResourceAttr("data.sigma_member.by_email", "member_type", "view"),
				),
			},
		},
	})
}

func TestAccMemberDataSource_byEmail_caseInsensitive(t *testing.T) {
	email := acctest.AccRandName("Member") + "@Example.Invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_member" "seed" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "MemberDSCase"
  member_type = "view"
}

data "sigma_member" "by_email" {
  email      = lower(sigma_member.seed.email)
  depends_on = [sigma_member.seed]
}
`, email),
				Check: resource.TestCheckResourceAttrPair("data.sigma_member.by_email", "id", "sigma_member.seed", "id"),
			},
		},
	})
}

func TestAccMemberDataSource_byID(t *testing.T) {
	email := acctest.AccRandName("member") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "sigma_member" "seed" {
  email       = %q
  first_name  = "AccTest"
  last_name   = "MemberDSByID"
  member_type = "view"
}

data "sigma_member" "by_id" {
  id = sigma_member.seed.id
}
`, email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.sigma_member.by_id", "email", email),
					resource.TestCheckResourceAttr("data.sigma_member.by_id", "first_name", "AccTest"),
					resource.TestCheckResourceAttr("data.sigma_member.by_id", "last_name", "MemberDSByID"),
				),
			},
		},
	})
}

func TestAccMemberDataSource_notFound(t *testing.T) {
	email := acctest.AccRandName("nobody") + "@example.invalid"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
data "sigma_member" "missing" {
  email = %q
}
`, email),
				ExpectError: regexp.MustCompile(`Member not found`),
			},
		},
	})
}
