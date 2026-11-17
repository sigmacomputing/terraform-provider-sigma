// Package connections implements sigma_connection, sigma_connection_grant,
// and sigma_connection_path_grant.
//
// Sigma's POST /v2/connections request body has a polymorphic `details` field
// (oneOf per warehouse type: Postgres, Redshift, AlloyDB, BigQuery, Snowflake,
// MySQL, Databricks, etc., each with different credential shapes including
// nested oneOf for password vs. AWS Secrets Manager). Modeling every variant
// as a typed Terraform schema would 10x the surface; instead the provider
// accepts `details_json` as a JSON string and forwards it verbatim. Users
// build the right shape per Sigma's API docs. Phase 2 may add typed sub-blocks
// for the most common warehouses if there's demand.
//
// Behavior worth knowing (verified end-to-end against AWS US West sandbox):
//
//   - The API STRIPS credentials from GET responses. Reads cannot refresh
//     details_json; we leave the user's last-applied value in state.
//
//   - Importing an existing connection populates id/name/type/etc. from GET
//     but leaves details_json blank, so the next plan proposes a re-PATCH.
//     Document `lifecycle { ignore_changes = [details_json] }` to suppress.
//
//   - For Snowflake, the canonical field inside details is `account` (or
//     `server`, which works as a synonym). `host` is REJECTED for Snowflake
//     despite the OAS oneOf treating it as interchangeable. Sigma appends
//     `.snowflakecomputing.com` server-side, so pass the raw `<org>-<acct>`
//     form, never the FQDN.
//
//   - PATCH accepts partial bodies. Updating only `name` works without
//     re-sending `details`.
//
//   - Verified types via real warehouses: snowflake, databricks, postgres,
//     bigQuery, mysql. Redshift and alloydb verified protocol-compatibly via
//     a Postgres-protocol target. SQL Server / Azure SQL DB and Starburst
//     are NOT currently usable through `POST /v2/connections` — Sigma's
//     public REST API has gaps for those (SIG-99778). The UI works.
package connections

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*connectionResource)(nil)
	_ resource.ResourceWithConfigure   = (*connectionResource)(nil)
	_ resource.ResourceWithImportState = (*connectionResource)(nil)
)

func NewResource() resource.Resource { return &connectionResource{} }

type connectionResource struct{ c *client.Client }

type connectionModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	Type           types.String `tfsdk:"type"`
	DetailsJSON    types.String `tfsdk:"details_json"`
	UseOauth       types.Bool   `tfsdk:"use_oauth"`
	OauthClientID  types.String `tfsdk:"oauth_client_id"`
	OauthJwkSetID  types.String `tfsdk:"oauth_jwk_set_id"`
	RoleSwitching  types.String `tfsdk:"role_switching"`
	IsSample       types.Bool   `tfsdk:"is_sample"`
	IsAuditLog     types.Bool   `tfsdk:"is_audit_log"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *connectionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

func (r *connectionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `A Sigma data connection.

**Credentials in ` + "`details_json`" + ` are write-only.** They are sent to Sigma on create/update but never read back — the API strips them from GET responses.

**Importing an existing connection:** ` + "`details_json`" + ` cannot be recovered from the server. After import, the next plan will propose to re-PATCH ` + "`details_json`" + ` with the value from your config (this is harmless if you have the credentials, since the PATCH just re-sends them). To suppress that diff, add ` + "`lifecycle { ignore_changes = [details_json] }`" + ` to the imported resource.

**Snowflake host/account/server quirk:** despite Sigma's OpenAPI listing ` + "`host`, `account`, and `server`" + ` as interchangeable, in practice for Snowflake the canonical field is ` + "`account`" + ` and Sigma appends ` + "`.snowflakecomputing.com`" + ` server-side. Pass the raw ` + "`<orgname>-<accountname>`" + ` form (no FQDN suffix) — passing the FQDN double-appends and fails with ` + "`Could not connect to host`" + `. ` + "`server`" + ` works as a synonym for ` + "`account`" + `; ` + "`host`" + ` is rejected.

**Azure SQL DB / SQL Server is not currently API-manageable.** Sigma's UI distinguishes ` + "`Azure SQL DB`" + ` (EngineEdition 5) from ` + "`SQL Server 2022`" + ` (EngineEditions 1–4 with ProductMajorVersion 16.x, or 8 for Managed Instance) as two distinct connection types, but the public REST API exposes only ` + "`type: \"sqlserver\"`" + `. Posting to ` + "`/v2/connections`" + ` with this type returns ` + "`Could not connect to database`" + ` even with valid creds and an open firewall — Sigma's API is likely routing the request to the SQL Server 2022 driver regardless of the target's EngineEdition. Tracked at SIG-99778. Workaround: create SQL Server connections in the Sigma UI; this provider can manage everything else (members, teams, grants on those connections, etc.) while the API gap is closed.

**Starburst / Trino / Presto is not exposed by Sigma's public REST API.** Despite UI support, ` + "`POST /v2/connections`" + ` rejects these type values at schema validation. Tracked at SIG-99778.`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"organization_id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, Description: "Connection name."},
			"type": schema.StringAttribute{Computed: true,
				Description: "Warehouse type (e.g. `postgres`, `snowflake`, `bigQuery`). Derived from `details_json.type`."},
			"details_json": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON-encoded connection details object per Sigma's POST /v2/connections oneOf schema. Write-only: never returned by GET, so refresh leaves the state value unchanged (no drift detection on this field). For Snowflake, set `account` to the raw `<orgname>-<accountname>` (Sigma appends `.snowflakecomputing.com`). For Snowflake OAuth connections, set `roleSwitching` (`allRoles` or `disabled`) inside this blob to control whether users may switch roles when running queries; the configured value is surfaced read-only on `role_switching`.",
			},
			"use_oauth": schema.BoolAttribute{Computed: true},
			"oauth_client_id": schema.StringAttribute{Computed: true,
				Description: "OAuth client ID configured on this connection, if any. Read-only (not part of the create body)."},
			"oauth_jwk_set_id": schema.StringAttribute{Computed: true,
				Description: "JWK set signing JWT bearer assertions for this connection, if any. Read-only — set it via `jwkSetId` (with `useJwt = true`) inside `details_json`."},
			"role_switching": schema.StringAttribute{Computed: true,
				Description: "For Snowflake OAuth connections, whether users may switch roles when running queries (`allRoles` or `disabled`). Read-only — set it via `roleSwitching` inside `details_json`. Empty for non-Snowflake-OAuth connections."},
			"is_sample":    schema.BoolAttribute{Computed: true},
			"is_audit_log": schema.BoolAttribute{Computed: true},
			"created_at":   schema.StringAttribute{Computed: true},
			"updated_at":   schema.StringAttribute{Computed: true},
		},
	}
}

func (r *connectionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.c = c
}

func (r *connectionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var details any
	if err := json.Unmarshal([]byte(plan.DetailsJSON.ValueString()), &details); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("details_json"), "Invalid JSON", err.Error())
		return
	}
	body := map[string]any{
		"name":    plan.Name.ValueString(),
		"details": details,
	}
	var got connection
	if err := r.c.Do(ctx, "POST", "/v2/connections", body, &got); err != nil {
		resp.Diagnostics.AddError("Create connection failed", err.Error())
		return
	}
	connToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got connection
	err := r.c.Do(ctx, "GET", "/v2/connections/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read connection failed", err.Error())
		return
	}
	connToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state connectionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		body["name"] = plan.Name.ValueString()
	}
	if plan.DetailsJSON.ValueString() != state.DetailsJSON.ValueString() {
		var details any
		if err := json.Unmarshal([]byte(plan.DetailsJSON.ValueString()), &details); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("details_json"), "Invalid JSON", err.Error())
			return
		}
		body["details"] = details
	}
	if len(body) > 0 {
		if err := r.c.Do(ctx, "PATCH", "/v2/connections/"+client.EscapeID(state.ID.ValueString()), body, nil); err != nil {
			resp.Diagnostics.AddError("Update connection failed", err.Error())
			return
		}
	}
	var got connection
	if err := r.c.Do(ctx, "GET", "/v2/connections/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		resp.Diagnostics.AddError("Re-read after update failed", err.Error())
		return
	}
	connToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/connections/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete connection failed", err.Error())
	}
}

func (r *connectionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func connToState(c *connection, dst *connectionModel) {
	dst.ID = types.StringValue(c.ConnectionID)
	dst.OrganizationID = types.StringValue(c.OrganizationID)
	dst.Name = types.StringValue(c.Name)
	dst.Type = types.StringValue(c.Type)
	dst.UseOauth = types.BoolValue(c.UseOauth)
	dst.OauthClientID = types.StringValue(c.OauthClientID)
	dst.OauthJwkSetID = types.StringValue(c.OauthJwkSetID)
	dst.RoleSwitching = types.StringValue(c.RoleSwitching)
	dst.IsSample = types.BoolValue(c.IsSample)
	dst.IsAuditLog = types.BoolValue(c.IsAuditLog)
	dst.CreatedAt = types.StringValue(c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	// Note: details_json is not refreshed from the API (credentials aren't returned).
	// State retains the user's last-applied JSON. Drift in non-credential fields is
	// undetected by design; users can change them and re-apply.
}
