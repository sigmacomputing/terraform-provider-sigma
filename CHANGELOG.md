# Changelog

All notable changes to `terraform-provider-sigma` are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
this project follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## v0.15.1

### Security

- Builds use Go 1.26.11 (was 1.26.2), fixing standard-library advisories in `crypto/tls`, `crypto/x509`, `net/http`, `net/url`, `net/textproto` and `encoding/asn1` that the provider's HTTPS client reaches, and a `net/http` header race (GO-2026-6859). The `go` directive is now 1.26.0, which `golang.org/x/net` v0.60.0 requires.
- Dependencies: `google.golang.org/grpc` v1.83.2, `golang.org/x/net` v0.60.0, `golang.org/x/crypto` v0.57.0, `github.com/buger/jsonparser` v1.1.2.

### Changed

- Every resource page in the docs now shows how to import it, and the registry groups pages into sections.
- Releases are built and signed in CI, with a build provenance attestation per artifact.

## v0.15.0

### Added

- `sigma_api_connector_grant` — gives a member or team access to an API connector through `/v2/grants`. `permission` is `view` (the UI's **Can use**, the default) or `edit` (**Can manage**). Sigma rejects the connection-style `usage` on connectors.

## v0.14.1

### Security

- Credentials are refused on an https→http redirect to the same host; the check now compares scheme as well as host.
- TRACE logs redact `sigma_api_connector` header and query-param values.
- A refused off-origin redirect fails at once instead of being retried four times.

### Fixed

- Creating or deleting a `sigma_member` refreshes the cached member list, so `sigma_member_account_type` sees the change in the same run.
- `sigma_plugin.url` docs: removing `url` keeps the current value; set `""` to clear it.

## v0.14.0

### Upgrade notes

- `base_url` must now use `https`; `http` is accepted only for localhost.
- `sigma_api_connector.params_json` and `data.sigma_team_user_attributes` values are now Sensitive. Any `output` that exposes them must set `sensitive = true`, or plan fails.

### Added

- `sigma_jwk_set` and `sigma_jwk_set_key` — signing keys for JWT bearer connections, with rotation (spec 20261030).
- `sigma_connection`: read-only `oauth_jwk_set_id`.
- `data.sigma_account_types` — all account types, built-in and custom, with ids.
- `data.sigma_whoami` — authenticated principal's user and organization ids.

### Fixed

- List calls on endpoints paged with `pageToken` now follow `nextPageToken`; they previously read only the first page. Affects `sigma_account_type`, `sigma_deployment_policy_file`, `sigma_deployment_policy_tenant`, `sigma_allowed_ip`, and the `account_types`, `api_connectors`, `api_connector_credentials`, `plugins`, `allowed_ips`, `deployment_policy_tenants` and `connection_table_columns` data sources. A resource past the first page could drop out of state and be recreated.
- `sigma_plugin`: changing `url` now replaces the plugin. Sigma's API can't update it in place, so the change was silently dropped and planned again on every apply.
- `sigma_member_account_type`: a member created earlier in the same apply is now found; the cached roster used to miss it and fail with "Member not found".

### Changed

- Fewer API calls. Lists request the largest page each endpoint allows (1000, or 500 for deployment policies and SAML), so an org with 5,000 members reads its roster in 5 calls instead of 100. `data.sigma_member` (by email) and `data.sigma_connection` (by name) filter server-side. Tag, team and member updates use the PATCH response instead of re-reading. JWK key lists are shared by every key and set in a run. Grant creation stops listing once the new grant appears.
- HTTP connections are pooled across Terraform's parallel operations instead of re-handshaking, and token fetches time out after 30s.
- Retries: a 429 `Retry-After` of up to 60s is waited out (was capped at 30s, so the retry came too early); a POST is no longer retried on 500 or 504, which could create duplicates.
- Request and response bodies are only masked for logging when TRACE logging is on.

### Security

- TRACE logs: request and response bodies are now redacted by parsed JSON key, covering secrets the old pattern missed (escaped service-account JSON including `private_key`, object-valued `password`, `apiKey` objects, bearer `token`, AWS `secretAccessKey`/`sessionToken`).
- The bearer token and client secret are only sent to the configured API host, and redirects to another host are refused. A refused redirect is now an error instead of a silent success.
- `base_url` must use `https` (`http` is allowed only for localhost).
- `sigma_api_connector.params_json` and `data.sigma_team_user_attributes` values are now Sensitive. Outputs that expose them need `sensitive = true`.
- `reassign_to_account_type_id` is query-escaped on account type delete.

## v0.13.0

### Added

- `sigma_smtp_settings` — custom SMTP server (password write-only).
- `sigma_storage_integration` + `data.sigma_storage_integrations` — S3/GCS/Azure storage integrations.
- `sigma_user_attribute_tenant_assignment` and `sigma_tenant_deployment_capability`.
- `sigma_saml_service_provider_certificate` + `data.sigma_saml_service_providers` (Beta API).
- `sigma_member`: optional `language` and `format_region` (Beta; spec 20261022).
- Spec synced to `2.0.0+20261008.88c408e`.

### Changed

- Client: a 429 with a long `Retry-After` (over 60s) now fails fast with the wait shown instead of retrying; after retries are exhausted the API's error body is returned instead of "giving up after N attempts".

## v0.12.0

### Changed

- `sigma_account_type`: renaming a type (changing `name`) no longer deletes it and drops its members to the delete fallback. Sigma has no rename endpoint, so the provider creates the new type, moves every member across (`PATCH /v2/members/{id}`), then deletes the old one. Each step is safe to re-run, so a failed apply is finished by applying again. **The type's `id` changes.** New optional `migrate_on_rename` (default `true`); set `false` for a plain replace. The plan carries a warning that a new id is created and that settings referencing the old type by id or name (e.g. SSO/SCIM role mappings) won't follow. A `description`-only change still replaces the type (Sigma would need two types with the same name).

## v0.11.0

### Changed

- `sigma_account_type.permissions` now updates **in place** via `PATCH /v2/accountTypes/{id}/permissions` (add/remove) instead of replacing the account type. Replacing a type reassigns its members to `reassign_to_account_type_id`, so this removes that side effect for permission edits. `name` and `description` still replace (Sigma has no update endpoint for them).

## v0.10.0

### Added

- **`sigma_member_account_type`** — assigns an account type to an EXISTING member without owning its
  lifecycle, for orgs where an IdP (e.g. Okta) creates and deactivates users. Never creates or
  deactivates a member; Create errors if the member doesn't exist; Destroy demotes to `reset_to`
  (default `Pro`); a member deactivated out of band drops out of state. Import by email.
- `sigma_deployment_policy.use_dependencies_workspace` — place implicit
  dependencies in a separate workspace (spec `2.0.0+20260930.a3bda220`).

### Fixed

- `sigma_account_type` Read and `sigma_account_type_permissions` no longer fail with "Duplicate Set
  Element" when the API returns a repeated permission name.

### Changed

- `sigma_organization_settings.ai_chat_history.retention_period` docs now list
  `never`. `make generate` points at `fern/apis/sigma-rest-api/openapi.json`.

## v0.9.0

### Added — `sigma_organization_settings` and deployment-policy resources

- **`sigma_organization_settings`** — singleton resource managing 10 org-wide
  settings sub-endpoints (`audit_logging`, `bulk_copy`, `comments`, `csv_upload`,
  `license_upgrade_requests`, `public_embeds`, `timezone`, `ai_chat_history`,
  `email_branding`, `sample_connections`) as independent, Optional+Computed
  nested blocks so any subset can be adopted piecemeal. Fixed resource ID
  `organization`; Delete only removes it from Terraform state — Sigma has no
  API to reset or delete organization settings as a whole (`email_branding`
  has its own DELETE endpoint, not exposed here for the same reason).
- **`sigma_deployment_policy`** (full CRUD), **`sigma_deployment_policy_file`**,
  and **`sigma_deployment_policy_tenant`** (association resources, composite IDs
  `<deployment_policy_id>:<inode_id>` / `<deployment_policy_id>:<tenant_organization_id>`)
  — manage Sigma deployment policies and their file/tenant associations.
  `version_tag_id` is immutable (`RequiresReplace`) — Sigma's update endpoint does
  not accept it.
- **`data.sigma_deployment_policy_tenants`** — lists tenant orgs available to
  target with a deployment policy, for looking up `tenant_organization_id` by
  slug or name.

### Removed — `sigma_workbook_grant` and `sigma_workbook_version_tag` resources

- ropped both workbook-scoped resources; the provider no longer manages anything
  under `sigma_workbook`. `data.sigma_workbook` / `data.sigma_workbooks` are unaffected.

## v0.8.1

### Fixed — O(N²) refresh in grant / assignment / account-type / tag / allowed-IP reads

- Extends the issue #24 team-membership fix to every resource with the same shape.
  These resources have no single-item GET endpoint, so each one's Read listed an
  entire parent or global collection to find itself; with N resources sharing a
  parent (or the same global list), every refresh/import was O(N²) API calls and
  rate-limited out on large estates.
- Added a generalized per-run list cache on the shared client
  (`client.CachedList` / `client.InvalidateList`, the generalization of the team
  roster cache). Each collection is now listed once per parent per run and reused;
  writes invalidate it. Affected reads: `sigma_connection_grant`,
  `sigma_connection_path_grant`, `sigma_workspace_grant`, `sigma_workbook_grant`,
  `sigma_user_attribute_assignment`, `sigma_team_attribute_assignment`,
  `sigma_account_type`, `sigma_tag`, `sigma_allowed_ip`. No behavior or schema
  change — purely fewer API calls.

## v0.8.0

### Added — `sigma_plugin` resource and `data.sigma_plugins` (beta)

- **`sigma_plugin`** — manage Sigma custom plugins via the beta `/v2/plugins` API (full CRUD).
  Attributes: `name` (required), `description`, `url`, `dev_url` (optional/computed — Sigma loads
  the plugin from `url` when it's added to a workbook and from `dev_url` in development mode;
  both default server-side when omitted), and computed `id` / `type`.
- **`data.sigma_plugins`** — lists every custom plugin in the org (resolve a plugin's `id` by name).

### Changed

- **`sigma_member.member_type` is now optional.** Sigma's `POST /v2/members` made `memberType`
  optional; when omitted, the org's Invitation default account type is applied and read back into
  state. Existing configs that set `member_type` are unaffected.
- **`sigma_connection` exposes a computed `role_switching`** attribute, surfacing the Snowflake-OAuth
  `roleSwitching` value (`allRoles` / `disabled`). It remains settable via `roleSwitching` inside
  `details_json`; this just makes the configured value visible.

### Fixed (issue #24) — `sigma_team_membership` import/refresh no longer O(N²) per team

- Reading a `sigma_team_membership` had to list the team's **entire** roster (Sigma has no
  single-membership read endpoint), and the roster wasn't cached — so N memberships on one team
  caused N full-roster lists per run (O(N²)), rate-limiting out on large teams (a ~1,000-member
  team needed ~23k API calls and was effectively un-importable).
- The roster is now listed **once per team per run** and reused across all of that team's
  memberships (O(N²) → O(N)), with per-team locking so concurrent reads coalesce into a single
  list and writes invalidate the cache.

### Removed — `sigma_data_model` resource

- Removed the `sigma_data_model` resource and its entire `internal/services/datamodels`
  package, docs (`docs/resources/data_model.md`), example, and acceptance/drift tests.
  Data models are an analytics-authoring concern that belongs in Sigma's own tooling, not
  in Terraform — managing them as code added a large, spec-coupled surface (typed `pages`,
  server-side ID remapping, unresolvable-column handling) without a clear infrastructure-as-code
  use case. The provider now focuses on org/admin and content-deployment resources. Connection
  data sources that previously referenced `sigma_data_model` no longer mention it.

## v0.7.0

### Added (issue #18) — connection data sources

- **`data.sigma_connections`** — lists all connections in the org (`id`, `name`, `type`,
  `is_sample`, `is_audit_log`, `use_oauth`), so a config can resolve a connection's UUID by name
  (e.g. `one([for c in data.sigma_connections.all.connections : c.id if c.name == "Snowflake"])`)
  instead of hardcoding it.
- **`data.sigma_connection`** — looks up a single connection by `name` or `id` (set exactly one).
  Name lookups error if the name isn't unique (use `id` to disambiguate). Returns the same fields.
- Fills the gap where the only connection data sources (`connection_path`, `connection_table_columns`)
  both required the `connection_id` you were trying to find — handy for pointing `sigma_data_model`,
  `sigma_connection_grant`, or `sigma_connection_path` at a UI-created / cross-root connection.

## v0.6.0

### Added (issue #15) — graceful handling of unresolvable columns in `sigma_data_model`

- New opt-in attribute `on_unresolvable_column` on `sigma_data_model`, one of `error` (default —
  unchanged behavior) or `skip`. In `skip` mode, when Sigma rejects the spec because a column
  isn't resolvable in the connection's catalog yet (e.g. the warehouse table hasn't been rebuilt),
  the provider strips the offending column(s) — plus any metrics, relationships, filters, sort,
  order, summary, groupings, folders, and column-security entries that reference them — and
  re-submits, bounded by a retry cap. Skipped columns surface as Terraform **warnings** (never
  silently dropped) and are retried automatically on the next apply once the catalog catches up.
  The apply still hard-errors if an entire table element resolves to zero columns. Mirrors the
  degrade-and-retry behavior of the internal dbt→Sigma sync this resource replaces.
- Improved the default (`error`-mode) failure message to **name the unresolvable column(s)** and
  hint at syncing the connection catalog / checking `information_schema.columns`, instead of
  surfacing the raw Sigma error.

## v0.5.2

### Fixed (issue #14) — grant resources no longer orphan grants on multi-grant applies

- The grant resources (`sigma_workspace_grant`, `sigma_workbook_grant`,
  `sigma_connection_grant`, `sigma_connection_path_grant`) confirm a create by re-listing
  grants — Sigma's `POST .../grants` returns an empty body, so the new `grantId` has to be
  discovered from the grant list. That list is **eventually consistent**: under a single
  `apply` creating several grants at once, the just-created grant often wasn't visible in the
  immediate read-back, so the provider errored without recording it — leaving a grant that
  exists in Sigma but is untracked by Terraform (and re-apply re-POSTed a duplicate rather than
  converging).
- The post-create read-back now **retries with backoff** (new `client.Eventually` helper) until
  the new grant appears, tolerating the consistency lag instead of erroring on the first miss.
- Create is now **idempotent**: if a grant matching the same grantee + permission already exists,
  it is adopted into state instead of POSTing a duplicate, so re-applying over a previously
  stranded grant converges. Manual UI cleanup is no longer required to recover.
- No schema, enum, or behavior change for normal single-grant applies.

## v0.5.1

### Docs (issue #12) — no behavior change

- Corrected the documented permission enums to match the API/schema:
  - `sigma_workspace_grant`: `view` / `explore` / `organize` / `edit` (the README wrongly
    listed `account` and omitted `organize`).
  - `sigma_connection_grant`: `usage` / `annotate` (the README wrongly said `editConnection`).
- Documented the surprising **enum → Sigma UI-label mapping** for `sigma_workspace_grant`
  in the resource schema description and README: `organize` = **Can contribute**,
  `edit` = **Can manage** (the top two are easy to get backwards). The accepted values
  were already correct — this is purely documentation.

## v0.5.0

### Added

- `sigma_member_account_type` — assigns an account type to an EXISTING member without owning its lifecycle (for IdP-managed orgs). Never creates or deactivates a member; destroy demotes to `reset_to` (default `Pro`); a member deactivated out of band drops out of state. Import by email.
- **Restored the full version-tag story** (removed in v0.4.0, brought back by demand —
  version tags are used to mark/pin published workbook & report versions):
  - `sigma_tag` — version-tag definition (`name` + `color` + `description`; `color`
    defaults to `cyan` when unset).
  - `sigma_workbook_version_tag` — applies a tag to a specific workbook version
    (references an existing workbook via `data.sigma_workbook`).
  - `data.sigma_tags` and `data.sigma_tag_workbooks` lookups.

  Counts: **24 resources + 19 data sources** (was 22 + 17). Note: version tags *label*
  a version — they do not swap connections; use `sigma_source_swap_policy` for that.

## v0.4.0

### Removed (BREAKING)

Scoped the provider to declarative org/access/content-as-code, dropping imperative
one-shot actions and workbook content/delivery management. Removed **8 resources**:

- **One-shot triggers** (imperative actions better suited to CI/CD orchestration):
  `sigma_data_model_materialize`, `sigma_data_model_swap_sources`,
  `sigma_template_swap_sources`, `sigma_webhook_trigger`.
- **Workbook content/delivery/tags**: `sigma_workbook_embed`, `sigma_workbook_schedule`,
  `sigma_workbook_version_tag`, and `sigma_tag` (version-tag definition, which only fed
  `version_tag`).

Also removed the now-orphaned **tag data sources** `data.sigma_tags` and
`data.sigma_tag_workbooks`.

**Kept:** `sigma_workbook_grant` (access governance) and `data.sigma_workbook` /
`data.sigma_workbooks` for referencing UI-authored workbooks; `sigma_data_model`,
`sigma_source_swap_policy`, and the template data sources. **Counts: 22 resources +
17 data sources** (was 30 + 19).

**Migration:** remove these resource/data-source blocks from configs and
`terraform state rm` them. For source swaps/materialization, run them as a pipeline
step (e.g. a `curl`/CLI call or `terraform apply -replace`) instead.

## v0.3.0

### Removed (BREAKING)

- **Removed the 9 workbook/data-model introspection data sources:**
  `sigma_workbook_elements`, `sigma_workbook_columns`, `sigma_workbook_pages`,
  `sigma_workbook_lineage`, `sigma_workbook_version_history`, `sigma_workbook_sources`,
  `sigma_data_model_elements`, `sigma_data_model_columns`, `sigma_data_model_sources`.
  These read-only "inspect the internals" lookups weren't core to managing a Sigma org
  as code. **Kept:** `data.sigma_workbook` / `data.sigma_workbooks` (primary lookups) and
  all workbook management resources (`_grant` / `_embed` / `_schedule` / `_version_tag`),
  plus the `sigma_data_model` resource. Data-source count is now 19 (was 28).

## v0.2.0

### Removed (BREAKING)

- **`sigma_workbook`** and **`sigma_workbook_from_template`** are removed. Terraform
  should not create workbooks: `sigma_workbook` only produced a *blank* workbook (content
  is authored in the Sigma UI) and the API has no workbook DELETE, so destroy only dropped
  it from state. Manage **existing** workbooks via `data.sigma_workbook` plus
  `sigma_workbook_grant` / `_embed` / `_schedule` / `_version_tag`. **Migration:** remove
  these resources from your config and state (`terraform state rm`); any workbooks they
  created remain in Sigma untouched. Resource count is now 30 (was 32).

### Fixed

- **Drift handling for archived inodes and grant permission changes** (PR #7): folders/
  workspaces that Sigma archives on delete, and connection/workbook/workspace grant
  permission updates, now reconcile correctly instead of showing perpetual or missed drift.
  Touches `sigma_folder`, `sigma_workspace`, `sigma_workspace_grant`,
  `sigma_connection_grant`, `sigma_connection_path_grant`, `sigma_workbook_grant`,
  `sigma_account_type`, `sigma_api_credential`, `sigma_data_model`, and the shared
  `internal/client` error handling.

## v0.1.2

### Changed

- **`sigma_tag`: omitting `color` now defaults to `cyan` instead of erroring.** The
  Sigma API rejects a tag create without a color, so the provider supplies `cyan`
  when `color` is not set (reverts the v0.1.0/v0.1.1 "color is required" behavior).
  Backward compatible — configs that set `color` are unaffected; configs that omitted
  it (which previously failed) now apply cleanly. Verified live (`TestAccTag_create_without_color`).
- Doc refresh: `sigma_member.member_type` guidance (use the stored license name, e.g.
  `build`, not the `creator` UI label) and the `sigma_connection_path_grant` example
  (dot-joined path) now appear in the generated docs.

## v0.1.1

Synced the bundled OpenAPI spec baseline forward (102 upstream commits;
`sigma-docs` `a363f5cd` → `72104400`, 2026-05-28) and added the new **API
connectors** surface. All changes are additive and non-breaking.

### Added (Wave 8)

- **`sigma_api_connector_credential`** — outbound auth for API connectors via
  `/v2/api-credentials` (full CRUD). The `credential` oneOf is accepted as a
  write-only, sensitive `credential_json` string (never returned by GET, so no
  drift detection — same convention as `sigma_connection.details_json`).
  `allowlist` (host globs) and `name`/`description` are refreshed from GET.
  This is intentionally **separate** from `sigma_api_credential`, which models
  OAuth for Sigma's own REST API (`/v2/credentials`) and is unchanged.
- **`sigma_api_connector`** — defines how Sigma calls an external HTTP endpoint
  via `/v2/api-connectors` (full CRUD). `params_json`/`config_json` are opaque
  JSON strings (no drift detection); `auth_id` references a
  `sigma_api_connector_credential`.
- **`data "sigma_api_connectors"`** and **`data "sigma_api_connector_credentials"`**
  — list views (secrets never surfaced).

### Changed

- **`sigma_connection`** gains a computed `oauth_client_id` (read-only; surfaced
  from the connection GET response — not part of the create body).

### Notes

- A planned `parent_source_url` field for the swap-sources resources was dropped:
  the spec's new `parentSourceUrlId` is a files/inode lineage attribute (on
  `/v2/files`), not a swap-sources field. Modeling it on a file-backed resource
  is deferred.
- Deferred from this release: `sigma_organization_settings`
  (`PATCH /v2/organizations/settings` has no GET, so no drift detection) and a
  dbt-artifacts upload trigger (`multipart/form-data`, needs new client support).

## v0.0.1 — Unreleased (internal test build)

### Added (Wave 7)

- **`data "sigma_member"`** — singular member lookup by `id` or `email`. Email matches are case-insensitive and short-circuit on the first hit via `client.ForEachPaged`. Designed to remove the "opaque memberId" footgun in HCL: instead of pasting Sigma's UUID, write `data.sigma_member.zen.id` where you need a `member_id`. Mirrors the existing `data.sigma_workbook` dual-mode (id-or-friendly-attr) precedent. Three acctests: `TestAccMemberDataSource_byEmail`, `_byEmail_caseInsensitive`, `_byID`, plus a not-found error path.

### Fixed (Wave 6)

- **`sigma_data_model` element-ID stability**: previously, every `terraform apply` caused Sigma to regenerate every element's internal ID (page, table, column, relationship, metric, folder, grouping, column-security), because the provider always sent user-supplied IDs and Sigma rewrote them server-side. This broke any downstream object (workbook, saved exploration, report) that referenced an old internal ID — confirmed via experiments against the tejas org. The provider now stores a user→server ID mapping in resource private state (invisible to schema/plan output) and sends server-assigned IDs on subsequent PUTs so Sigma preserves identity. Verified via three new acctests: `TestAccDataModel_idStability_changeFormula`, `_changeName`, `_addColumn`. Renaming a user-supplied `id` in HCL still causes a one-time regen for that element — pick stable IDs up front.
- **Migration note for existing v0 users**: the first `terraform apply` after upgrading captures the current server IDs into private state. The first apply behaves like the old (regen-on-every-PUT) behavior; every subsequent apply is stable. We recommend running a no-op apply (or `terraform refresh`) immediately after upgrading to lock in the mapping before making any meaningful changes.


Feature-complete for identity, connections, workspaces, workbook sharing,
folders, tenants, source-swap policies, and data models. Internal-test snapshot;
not yet published to the Terraform Registry.

### Added

- 30 resources covering members, teams, account types, user attributes, API
  credentials, connections + grants, workspaces + grants, tags, workbook
  grants/embeds/version-tags/schedules, workbook resource, data model + the
  per-data-model `materialize` and `swap_sources` one-shot triggers, folder,
  tenant, source-swap policy, IP allowlist entries, shared-template
  acceptance, one-shot webhook triggers, and template-driven workbook
  creation + per-template source swap. See
  [Resources](README.md#resources-30) for the full list with per-resource notes.
- 26 data sources for read-only lookups across the same surfaces (including
  3 data sources for inspecting a data model's elements / columns / sources,
  6 for inspecting a workbook's elements / columns / pages / lineage /
  version-history / sources, IP allowlist, and shared/own templates). See
  [Data sources](README.md#data-sources-25).
- `sigma_workbook` resource — creates a workbook in a folder; no-op destroy (API has no DELETE).
- `sigma_workbook_schedule` resource — full CRUD delivery schedules (email, Slack, webhook, GSheets, GDrive) on workbooks. `targets` is a typed list (each entry sets one of email/team_id/slack_conversation_id/google_spreadsheet_url/google_drive_folder_url/webhook_url); `config_json` remains opaque due to deep export-format polymorphism.
- `sigma_data_model` resource — full CRUD with first-class typed schema covering the documented Sigma data-model spec end-to-end. Typed `pages` blocks expose all 7 source kinds (`warehouse_table`, `sql`, `table_reference`, `data_model_reference`, `join`, `union`, `transpose`), columns with number/datetime format variants, relationships with key pairs, 5 typed table filter kinds (`number_range`, `date_range` with 8 modes, `top_n`, `list`, `text_match`), 7 optional table fields (`metrics`, `folders`, `groupings`, `summary`, `order`, `sort`, `column_securities` with 3-criteria discriminator), and 14 typed controlType configs (checkbox, switch, text, text-area, number, number-range, date, date-range, top-n, list, segmented, slider, range-slider, hierarchy) including typed control filters and parameters. `spec_json` remains as a forward-compat escape hatch (mutually exclusive with `pages`) for OAS additions not yet typed. Metadata changes via `PATCH /v2/files/{id}`; spec changes via `PUT /v2/dataModels/{id}/spec`. Destroy calls `DELETE /v2/files/{id}`.
- `sigma_data_model_materialize` resource — one-shot trigger for `POST /v2/dataModels/{id}:materialize`. Each apply fires a fresh run; use `triggers` to wire re-runs to upstream changes.
- `sigma_data_model_swap_sources` resource — one-shot per-data-model source swap via `PUT /v2/dataModels/{id}/swapSources`. Distinct from the org-level `sigma_source_swap_policy`. Typed `source_mapping` and `connection_mapping` lists.
- `sigma_data_model_elements` / `_columns` / `_sources` data sources — read-only views for content-deployment workflows, paginated over the corresponding GET endpoints.
- `sigma_allowed_ip` resource + `sigma_allowed_ips` data source — manage entries in the org's IP allowlist (Sigma beta API). Resource wraps the batch-only API by sending one-element batches per Create/Destroy; `scope` enum is `public-api` / `ui` / `both`.
- `sigma_shared_template_acceptance` resource + `sigma_shared_templates` data source — accept / reject workbook templates shared cross-org. Listing returns pending and accepted shares with `pending` boolean.
- `sigma_webhook_trigger` resource — one-shot trigger for Sigma's beta webhook endpoint (`POST /v2/webhooks/{workbookId}/{sequenceId}`). Body is opaque (matches the configured webhook spec); `triggers` map forces replace and re-fires.
- `sigma_workbook_from_template` resource — creates a workbook from a Sigma workbook template via `POST /v2/templates/save_workbook`. The resulting workbook is independent (manage it via `sigma_workbook` after creation). Supports `update_automatically` to keep the workbook synced with template changes.
- `sigma_template_swap_sources` resource — one-shot per-template source swap via `POST /v2/templates/{id}/swapSources`. Same typed schema as `sigma_data_model_swap_sources`.
- `sigma_templates` / `sigma_template` data sources — list all workbook templates in the org / look up a single template by ID.
- 6 new workbook read-only data sources for content-deployment workflows: `sigma_workbook_elements`, `sigma_workbook_columns`, `sigma_workbook_pages`, `sigma_workbook_lineage`, `sigma_workbook_version_history`, `sigma_workbook_sources`. Useful for impact analysis and source-aware deployments.
- `sigma_folder` resource — full CRUD folder management via `/v2/files`; destroy archives content (soft-delete).
- `sigma_tenant` resource — full CRUD tenant org provisioning for embedded analytics.
- `sigma_source_swap_policy` resource — full CRUD source-swap policies for environment promotion and multi-tenant content deployment. Typed `connection_swaps` list (each with `to_connection_id` plus `path0`/`path1`/`path2` blocks; `swap_type` discriminator: `exact`/`attribute`/`all`).
- `send_invite` boolean on `sigma_member` for silent provisioning (skip the
  invitation email at create time).
- Self-bootstrapping `make docs` and `make generate` — pinned codegen and
  documentation tools resolve through `go run` against `go.mod`, so contributors
  don't need anything on `PATH` beyond Go itself.

### Security

- **M1**: `Sensitive: true` on `sigma_user_attribute.default_value`,
  `sigma_user_attribute_team_assignment.value`, and
  `sigma_user_attribute_user_assignment.value`. UA values commonly carry
  tenant IDs, region scopes, or PII-adjacent identifiers used for row-level
  security; they no longer print in plan output or state diffs.
- **M2**: `sigma_api_credential` description now carries an explicit
  encrypted-state-backend warning, and the README has a dedicated
  [State file security](README.md#state-file-security) section enumerating
  every field that holds sensitive material.
- **M3**: HTTP body logging at TRACE now masks UA value envelopes
  (`"value":{"type":"string","val":"..."}` and `"defaultValue":...`). Schema
  `Sensitive` does not extend to body logs; this regex covers the gap.
- **L1**: `client.EscapeID()` now wraps every state-derived ID concatenated
  into a URL path (61 sites across 21 files). Defense-in-depth — Sigma's
  IDs are alphanumeric-with-hyphens in practice, so this is a no-op for
  valid IDs.

### Notes

- `sigma_workbook` destroy is a no-op — the API has no DELETE for workbooks. TF state is cleaned up; the workbook remains in Sigma.
- `sigma_data_model` is fully CRUD — all attributes updatable in-place. Destroy calls `DELETE /v2/files/{id}` and permanently removes the model.
- Read [Lifecycle nuances](README.md#lifecycle-nuances) before applying against
  production. Key surprises: `sigma_member` (destroy archives; create sends email), `sigma_workspace` (destroy is unrecoverable), `sigma_folder` (destroy archives, not hard-delete), `sigma_workbook_embed` (destroy invalidates live URLs), `sigma_api_credential` (destroy revokes immediately).
- Resources cut from v0 because they aren't yet end-to-end verified or are
  blocked on Sigma API gaps are tracked in [Not in v0 (deferred)](README.md#not-in-v0-deferred).
