---
page_title: "Getting started with the Sigma Terraform provider"
subcategory: "Guides"
description: |-
  How to authenticate, install the Sigma Terraform provider, and write a first configuration.
---

# Getting started with the Sigma Terraform provider

This guide is for anyone who wants to manage a Sigma organization as code. It
covers authentication, installation, and a first working configuration.

> The provider currently ships **35 resources** and **27 data sources** covering
> identity & access, connections & integrations, workspaces & content, organization
> settings, and tenants & deployment. See the [provider docs](https://registry.terraform.io/providers/sigmacomputing/sigma/latest/docs) and
> the [README](https://github.com/sigmacomputing/terraform-provider-sigma#readme) for the full list and per-resource lifecycle notes.

---

## 1. Authenticate

The provider uses Sigma's OAuth2 client-credentials flow. Create a client in the
Sigma UI under **Administration → Developer Access**, then provide three values:

| Setting | Env var | Default |
|---|---|---|
| `client_id` | `SIGMA_CLIENT_ID` | — (required) |
| `client_secret` | `SIGMA_CLIENT_SECRET` | — (required) |
| `base_url` | `SIGMA_BASE_URL` | `https://api.sigmacomputing.com` |

Set `base_url` to your region's API host (e.g. AWS US-West, EU/London, AU). The
`client_secret` is marked sensitive and is never written to state or logs.

```hcl
provider "sigma" {
  # Prefer env vars in CI: SIGMA_CLIENT_ID / SIGMA_CLIENT_SECRET / SIGMA_BASE_URL
  client_id     = var.sigma_client_id
  client_secret = var.sigma_client_secret
  base_url      = "https://api.sigmacomputing.com"
}
```

---

## 2. Install the provider

The provider is published to the Terraform Registry as `sigmacomputing/sigma`:

```hcl
terraform {
  required_providers {
    sigma = {
      source  = "sigmacomputing/sigma"
      version = "~> 0.16"
    }
  }
}
```

### Air-gapped environments

Download a release zip from GitHub and place the binary in the [implied local mirror][mirror] layout:

```
~/.terraform.d/plugins/registry.terraform.io/sigmacomputing/sigma/<version>/<os>_<arch>/terraform-provider-sigma_v<version>
```

For shared CI, host the same directory tree behind HTTP and point `~/.terraformrc` at it with a
`provider_installation { network_mirror { url = "…" } }` block.

[mirror]: https://developer.hashicorp.com/terraform/cli/config/config-file#provider-installation

---

## 3. First configuration

```hcl
terraform {
  required_providers {
    sigma = {
      source  = "sigmacomputing/sigma"
      version = "~> 0.16"
    }
  }
}

provider "sigma" {
  client_id     = var.sigma_client_id
  client_secret = var.sigma_client_secret
}

# A team, a member, and the team membership tying them together.
resource "sigma_team" "analytics" {
  name = "Analytics"
}

resource "sigma_member" "jane" {
  email       = "jane@example.com"
  first_name  = "Jane"
  last_name   = "Doe"
  member_type = "build" # Sigma stores the "Creator" account type as "build"
}

resource "sigma_team_membership" "jane_analytics" {
  team_id   = sigma_team.analytics.id
  member_id = sigma_member.jane.id
}

# Look up existing workspaces without managing them.
data "sigma_workspaces" "all" {}
```

```bash
terraform init
terraform plan
terraform apply
```

See [`examples/`](https://github.com/sigmacomputing/terraform-provider-sigma/tree/main/examples) in the repo for one config per resource and data source.

---

## 4. Operational notes

- **State contains secrets.** `sigma_api_credential.client_secret`,
  `sigma_connection.details_json`, `sigma_api_connector_credential.credential_json`,
  `sigma_smtp_settings.password` and user-attribute values land in state. Use a
  remote backend with encryption (HCP Terraform, or S3 + KMS) and restrict access.
  See the README "State file security" section.
- **Lifecycle quirks.** Some resources are create-only / replace-on-change
  (e.g. grants, `sigma_account_type`, `sigma_shared_template_acceptance`). Opaque-JSON
  fields (`details_json`, `params_json`, `config_json`, `credential_json`) have no drift
  detection. Read the README "Resource lifecycle patterns" before relying on in-place updates.
- **Regions.** One provider block targets one region via `base_url`. Use multiple
  aliased provider blocks to manage more than one Sigma region from one config.
- **Support.** Open an issue in the provider's GitHub repository.
