# Sigma Terraform Provider

[![Tests](https://github.com/sigmacomputing/terraform-provider-sigma/actions/workflows/test.yml/badge.svg)](https://github.com/sigmacomputing/terraform-provider-sigma/actions/workflows/test.yml)
[![Latest release](https://img.shields.io/github/v/release/sigmacomputing/terraform-provider-sigma)](https://github.com/sigmacomputing/terraform-provider-sigma/releases/latest)
[![Terraform Registry](https://img.shields.io/badge/registry-sigmacomputing%2Fsigma-623CE4)](https://registry.terraform.io/providers/sigmacomputing/sigma/latest)
[![License](https://img.shields.io/github/license/sigmacomputing/terraform-provider-sigma)](LICENSE)

The Sigma provider for [Terraform](https://www.terraform.io) manages a [Sigma](https://www.sigmacomputing.com) organization through Sigma's [REST API](https://help.sigmacomputing.com/reference): members, teams and account types, warehouse connections and grants, workspaces and folders, user attributes, tenants, organization settings, and more.

Important documents: [Registry docs](https://registry.terraform.io/providers/sigmacomputing/sigma/latest/docs)
| [Getting started guide](docs/guides/getting-started.md)
| [Changelog](CHANGELOG.md)
| [Contributing](CONTRIBUTING.md)
| [Security policy](SECURITY.md)

> **Status:** the provider is pre-1.0. Resources and attributes can change between minor versions; breaking changes are called out in the [changelog](CHANGELOG.md). Pin a minor version (`~> 0.15`) and read the changelog before upgrading.

> ⚠️ **Security:** if you believe you have found a security issue, report it through Sigma's [Vulnerability Disclosure Program](https://www.sigmacomputing.com/product/vdp). Do not open a public GitHub issue.

## Support

Report bugs and request features in [GitHub issues](https://github.com/sigmacomputing/terraform-provider-sigma/issues). For questions about Sigma itself, see the [Sigma help center](https://help.sigmacomputing.com).

## Table of contents

- [Requirements](#requirements)
- [Getting started](#getting-started)
- [What you can manage](#what-you-can-manage)
- [Resource lifecycle patterns](#resource-lifecycle-patterns)
- [Lifecycle nuances](#lifecycle-nuances)
- [Data handling](#data-handling)
- [State file security](#state-file-security)
- [Logging](#logging)
- [Development](#development)
- [Not yet supported](#not-yet-supported)
- [License and Legal Notices](#license-and-legal-notices)

## Requirements

The provider is free to download and use. To do anything with it you need a Sigma subscription and API credentials for your organization:

- [Terraform](https://developer.hashicorp.com/terraform/install) **1.0** or later (the provider uses plugin protocol 6).
- A Sigma organization and an **API client ID and secret**, created under **Administration → Developer Access**. The credential acts with its owner's account type, so most resources need it to belong to an **Admin**.
- Your organization's **API base URL**, shown on the same page. It depends on your cloud and region (for example `https://aws-api.sigmacomputing.com`).

Release binaries are built for Linux, macOS and Windows on `amd64` and `arm64`, plus `386` for Linux and Windows.

## Getting started

```hcl
terraform {
  required_providers {
    sigma = {
      source  = "sigmacomputing/sigma"
      version = "~> 0.15"
    }
  }
}

# Or set SIGMA_BASE_URL, SIGMA_CLIENT_ID and SIGMA_CLIENT_SECRET instead.
provider "sigma" {
  base_url      = "https://aws-api.sigmacomputing.com"
  client_id     = var.sigma_client_id
  client_secret = var.sigma_client_secret
}

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
```

```bash
terraform init   # download the provider
terraform plan   # preview changes
terraform apply  # create the resources
```

Keep the client secret out of version control: pass it with `TF_VAR_sigma_client_secret` or `SIGMA_CLIENT_SECRET`, or read it from your secrets manager.

To bring existing Sigma objects under Terraform, every resource page in the registry docs has an **Import** section with the ID format. One provider block targets one organization; use [provider aliases](https://developer.hashicorp.com/terraform/language/providers/configuration#alias-multiple-provider-configurations) to manage several.

Next:

- [Getting started guide](docs/guides/getting-started.md): authentication, installation options and operational notes
- [Registry docs](https://registry.terraform.io/providers/sigmacomputing/sigma/latest/docs): the full resource and data source reference
- [`examples/`](examples/): one configuration per resource and data source

## What you can manage

Full reference: [registry docs](https://registry.terraform.io/providers/sigmacomputing/sigma/latest/docs). The groups below match the registry's sections.

| Area | Resources | Data sources |
|---|---|---|
| **Identity & Access** | `sigma_member`, `sigma_member_account_type`, `sigma_team`, `sigma_team_membership`, `sigma_account_type`, `sigma_api_credential`, `sigma_user_attribute`, `sigma_user_attribute_team_assignment`, `sigma_user_attribute_user_assignment`, `sigma_user_attribute_tenant_assignment` | `sigma_member`, `sigma_members`, `sigma_member_teams`, `sigma_teams`, `sigma_team_user_attributes`, `sigma_account_types`, `sigma_account_type_permissions`, `sigma_whoami` |
| **Connections & Integrations** | `sigma_connection`, `sigma_connection_grant`, `sigma_connection_path_grant`, `sigma_source_swap_policy`, `sigma_storage_integration`, `sigma_api_connector`, `sigma_api_connector_credential`, `sigma_api_connector_grant`, `sigma_jwk_set`, `sigma_jwk_set_key` | `sigma_connection`, `sigma_connections`, `sigma_connection_path`, `sigma_connection_table_columns`, `sigma_storage_integrations`, `sigma_api_connectors`, `sigma_api_connector_credentials` |
| **Workspaces & Content** | `sigma_workspace`, `sigma_workspace_grant`, `sigma_folder`, `sigma_tag`, `sigma_plugin`, `sigma_shared_template_acceptance` | `sigma_workspaces`, `sigma_workbook`, `sigma_workbooks`, `sigma_tags`, `sigma_tag_workbooks`, `sigma_template`, `sigma_templates`, `sigma_shared_templates`, `sigma_plugins` |
| **Organization** | `sigma_organization_settings`, `sigma_smtp_settings`, `sigma_allowed_ip`, `sigma_saml_service_provider_certificate` | `sigma_allowed_ips`, `sigma_saml_service_providers` |
| **Tenants & Deployment** | `sigma_tenant`, `sigma_tenant_deployment_capability`, `sigma_deployment_policy`, `sigma_deployment_policy_file`, `sigma_deployment_policy_tenant` | `sigma_deployment_policy_tenants` |

## Resource lifecycle patterns

Most resources here follow the standard "declare desired state, provider keeps API state in sync" Terraform model. A handful map to one-shot Sigma API actions and behave more like imperative triggers — they're still TF resources for ergonomics (typed arguments, plan diffs on input changes, no shell-script glue), but their lifecycle is non-standard. This section groups every resource by its lifecycle pattern so you can see at a glance what to expect.

### Standard CRUD

Behaves the way Terraform users expect: `terraform apply` creates, `terraform plan` detects drift, in-place updates work where the API supports them, `terraform destroy` removes the underlying object (or archives it where the API only allows soft-delete).

`sigma_member`, `sigma_team`, `sigma_team_membership`, `sigma_account_type`, `sigma_user_attribute`, `sigma_user_attribute_team_assignment`, `sigma_user_attribute_user_assignment`, `sigma_api_credential`, `sigma_connection`, `sigma_connection_grant`, `sigma_connection_path_grant`, `sigma_workspace`, `sigma_workspace_grant`, `sigma_tag`, `sigma_folder`, `sigma_tenant`, `sigma_source_swap_policy`, `sigma_allowed_ip`, `sigma_shared_template_acceptance`, `sigma_api_connector_credential`, `sigma_api_connector`, `sigma_api_connector_grant`.

Imperative one-shot actions (data-model/template source swaps, materialization, webhook triggers) were removed in v0.4.0 — those workflows belong in CI/CD orchestration (`terraform apply -replace=...` or a pipeline step), not as permanent TF resources.

## Lifecycle nuances

Things that aren't obvious from the schema. Read this before pointing the provider at production.

### `sigma_member`
- **Creating sends an email invite by default.** Sigma's `POST /v2/members` triggers an invitation email to the user (unless `user_kind = "embed"`). For silent provisioning — e.g. bulk imports where an external IdP handles user onboarding — set `send_invite = false`. The flag is **only consulted at create time**; toggling it on an existing member is a no-op (the invite either already happened or is no longer relevant). To suppress the diff after a post-creation flip, add `lifecycle { ignore_changes = [send_invite] }`.
- **Destroying deactivates, doesn't delete.** Sigma archives the member; their email is reserved and cannot be re-used for a new member without an admin restore in the Sigma UI. Useful for audit; surprising if you're trying to "reset" a test user.
- Email is mutable via `update`. Renaming a user via `email` change updates the existing record (does not create a new one).

### `sigma_team` / `sigma_team_membership`
- Destroying a `sigma_team_membership` only removes the link. The member and team both survive.
- Destroying a `sigma_team` does NOT delete its members. Team grants and UA team-assignments referencing it should be destroyed first to avoid 404s on next plan.

### `sigma_account_type`
- **No PATCH on Sigma's side**, so any change to `name`, `description`, or `permissions` forces replace. If members are currently assigned to that account type, the destroy step can leave them on a missing type briefly. Sequence with care, or ignore the diff with `lifecycle { ignore_changes = [permissions] }` if it's drift you don't want to chase.

### `sigma_user_attribute*`
- Precedence: a user assignment beats a team assignment beats `default_value`. Setting `sigma_user_attribute_user_assignment` for someone overrides whatever their teams say.
- Values are `Sensitive` — they don't print to plan output, state diffs, or HTTP body logs. Useful because UA values often hold tenant IDs, region scopes, or PII-adjacent identifiers used for row-level security.

### `sigma_connection`
- **Sigma validates connectivity at create time.** The credential in `details_json` must be live and reachable from Sigma's egress when you `apply` — there is no "create now, fix later" mode. A bad credential surfaces as an explicit error from Sigma at apply time.
- `details_json` is **write-only**. Sigma's GET strips credentials, so we never refresh from server. After import, the next plan proposes a re-PATCH; suppress with `lifecycle { ignore_changes = [details_json] }`.
- Destroying a connection cascades on Sigma's side — workbooks/datasets sourced from it will break.

### `sigma_connection_grant` / `sigma_connection_path_grant`
- `path` grants are atomic per (connection_path, grantee). The path tree is implicit on Sigma's side; the provider treats each grant as flat.
- Destroying a grant immediately revokes access — query history and existing scheduled exports stay, but new requests fail.

### `sigma_workspace`
- **Destroy is unrecoverable.** Sigma's docs explicitly note "Deleted workspaces cannot be recovered." Any documents inside that haven't been moved out are gone with the workspace. Treat workspace destroy as a real deletion, not a soft-delete.
- `sigma_workspace_grant` destroy just revokes access; workspace contents are unaffected.
- **Permission ↔ Sigma UI label mapping** (the `permission` value is the API/TF value, *not* the UI label — the top two are easy to get backwards):

  | `permission` | Sigma Share-dialog label |
  |---|---|
  | `view` | Can view |
  | `explore` | Can explore |
  | `organize` | **Can contribute** |
  | `edit` | **Can manage** |

### `sigma_api_credential`
- **`client_secret` is returned exactly once.** State is the only place it ever lives — losing or leaking state means rotating the credential. Use an encrypted state backend (see the next section).
- **Destroy revokes the credential.** Any tooling currently authenticating with this `client_id` will start receiving 401s within seconds. If the credential the provider itself is using is ever destroyed by a config it manages, that apply will fail mid-flight on the next call.
- Importing recovers `client_id` only — `client_secret` cannot be re-fetched. If you need a usable secret in state for an imported credential, taint and recreate.

### `sigma_folder`
- **Destroy archives, it does not hard-delete.** Content inside the folder is preserved and can be restored via the Sigma UI. This matches Sigma's own UI behavior for folder deletion.
- Moving a folder (changing `parent_id`) is a live PATCH — content moves with it.

### `sigma_tenant`
- `cloud_provider` is immutable after creation — changing it forces replacement of the entire tenant org. Defaults to the parent org's cloud provider if omitted.
- Name and slug are mutable in-place. Slug changes affect any tenant-specific URLs or embed configurations pointing at the old slug.

### `sigma_source_swap_policy`
- `type` is immutable after creation (currently only `"connection"` is supported).
- `connection_swaps` is a typed list — each entry has `to_connection_id` and three path components (`path0`, `path1`, `path2`). Each path picks a `swap_type`: `exact` (literal — set `catalog_object`), `attribute` (resolve via `user_attribute_id`), or `all` (pass-through; no other fields). The API rejects "all at all levels" — at least one path must be `exact` or `attribute`.

## Data handling

- **Where it runs.** The provider runs wherever you run Terraform (a laptop or your CI). Sigma doesn't host or operate it.
- **What it talks to.** It calls only the Sigma API host you set in `base_url`, over HTTPS, and refuses redirects to other hosts. It has no telemetry and sends nothing to Sigma or anyone else beyond those API calls.
- **What data comes out.** It reads the configuration and metadata of the objects you manage or look up: for example member names and emails, teams, account types, connection settings, workspace and folder names, grants, and user-attribute values. Terraform writes this to your state file, wherever your backend stores it.
- **What it doesn't read.** It doesn't read the data in your workbooks, data models, datasets or connected warehouses, or query any warehouse. Workbook and data model resources and data sources only handle metadata such as names, paths, owners and permissions.
- **Sensitive data.** Some values end up in state, including credentials you supply. See [State file security](#state-file-security) for the list.
- **Protecting your data.** Store state in an encrypted backend with restricted access, keep `client_id` and `client_secret` in a secret store or environment variables rather than in `.tf` files, use a dedicated API credential and rotate it, and treat `TF_LOG` output as sensitive even though known secret fields are masked.

## State file security

Several resources store sensitive material in Terraform state by design:

- `sigma_api_credential.client_secret` — returned by Sigma exactly once at create time. There is no GET endpoint; the only copy lives in your state.
- `sigma_connection.details_json` — warehouse credentials (passwords, private keys, service-account JSON) you supply at create time. Sigma's GET strips them; the only copy lives in your state.
- `sigma_user_attribute.default_value` and `sigma_user_attribute_*_assignment.value` — user-attribute values used for row-level security; commonly tenant IDs or PII-adjacent identifiers.

**Use an encrypted remote backend** for any production use of this provider:

- AWS S3 with KMS encryption + state locking via DynamoDB
- Terraform Cloud / Terraform Enterprise (encrypts at rest, audit logs)
- Azure Blob Storage with encryption at rest
- GCS with customer-managed encryption keys

Local-disk state (`terraform.tfstate` on a developer machine) is not suitable for any deployment that touches real credentials.

## Logging

`TF_LOG=DEBUG` produces per-call HTTP logs. The `sigma_http` subsystem carries
request/response bodies at TRACE level. Sensitive fields are masked both as
structured field values and as JSON body patterns:

- OAuth & API credentials: `Authorization`, `client_secret`, `access_token`, `refresh_token`
- Warehouse credentials: `password`, `privateKey`, `privateKeyPassphrase`, `serviceAccount`
- Generic: `secret`, `api_key`
- Embed tokens: `embedUrl` / `embed_url`
- User-attribute values: `value` / `defaultValue` / `default_value` (typed envelopes `{type, val}`)

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

### Local development

```bash
make install   # builds + installs into ~/.terraform.d/plugins
```

Add this to `~/.terraformrc` to use the local build:

```hcl
provider_installation {
  dev_overrides {
    "sigmacomputing/sigma" = "~/.terraform.d/plugins/registry.terraform.io/sigmacomputing/sigma/0.0.1/<os>_<arch>"
  }
  direct {}
}
```

Then run `terraform plan` against a config in `examples/`.

### Acceptance tests

```bash
export SIGMA_CLIENT_ID=...
export SIGMA_CLIENT_SECRET=...
export SIGMA_BASE_URL=...   # your sandbox region's API host
make acctest
```

These hit a real Sigma org — only run against a sandbox.

To also run the **warehouse-dependent** tests, provision a throwaway Snowflake
service account + scratch table and a matching Sigma connection. The script needs
a `snow` CLI connection with admin rights on a **test** Snowflake account:

```bash
export SNOW_CONN=<snow CLI connection name> SF_ACCOUNT=<org>-<account>
eval "$(./scripts/snowflake-test-sa.sh bootstrap)"   # sets SIGMA_TEST_SNOWFLAKE_* / _CONNECTION_ID / _CONNECTION_PATH
make acctest
./scripts/snowflake-test-sa.sh teardown              # drops everything when done
```

#### What's covered (verified 2026-06-04)

- **Base run (no fixtures): 38 passing, 0 failing.** Identity (members/teams/account
  types/user attributes), tags, workspaces + grants, folders, API credential, **API
  connectors + credentials** (incl. data sources), workbook create/grant/embed/schedule,
  plus committed **cross-resource relationship** and **drift-detection** tests
  (`internal/e2e/`).
- **With `snowflake-test-sa.sh bootstrap`:** the warehouse-gated tests also pass —
  `sigma_connection` (incl. the new `oauth_client_id`), connection + path grants,
  `data.sigma_connection_path`, and `sigma_source_swap_policy`.
- **Still skipped — need fixtures/features not scriptable here:**
  - `data.sigma_connection_table_columns` — needs a pre-resolved table UUID (catalog sync).
  - `sigma_tenant` (multitenancy off), `sigma_allowed_ip` (beta), and
    `sigma_shared_template_acceptance` (cross-org share).

### Architecture

- `terraform-plugin-framework` (not SDKv2).
- OAuth2 client-credentials via `golang.org/x/oauth2/clientcredentials`, single
  shared `oauth2.ReuseTokenSource` across all resources.
- `hashicorp/go-retryablehttp` with a custom policy that honors `Retry-After`
  on 429 and retries 5xx with jittered exponential backoff.
- `internal/client` is the only package that touches `net/http`.

## Not yet supported

Not available yet, because they aren't verified end to end or the Sigma API is missing
the endpoints they need:

- **Workbook bookmarks** — needs a workbook with a saved exploration to test against.
- **Folder data source by path** — only ID-based lookup is wired today; name-based path resolution deferred.
- **`sigma_report` resource** — same POST+GET-only API limitation as workbook had; deferred.
- **Cross-org workbook share** — POST-only action endpoint, not modelable as a TF resource.

## License and Legal Notices

Sigma Terraform Provider<br>
Copyright 2026 Sigma Computing, Inc.

Sigma Terraform Provider (the “application”) is licensed by Sigma Computing, Inc. (“Sigma”) under the [Apache License, Version 2.0](LICENSE) (the “License”).

The application is not a part of Sigma’s services provided under Sigma’s Main Services Agreement and other similar commercial agreements between Sigma and its customers. The provision, access, installation, download, and use of the application are governed solely by the License; and you may not use the application except in compliance with the License. Unless required by applicable law or agreed to in writing, the application is distributed on an “as-is” basis, without warranties or conditions of any kind, either express or implied. Your use of the application is at your own risk, and Sigma has no obligation to support your use of this application.
