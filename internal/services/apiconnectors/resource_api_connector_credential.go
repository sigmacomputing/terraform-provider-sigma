package apiconnectors

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
	_ resource.Resource                = (*credentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*credentialResource)(nil)
	_ resource.ResourceWithImportState = (*credentialResource)(nil)
)

func NewCredentialResource() resource.Resource { return &credentialResource{} }

type credentialResource struct{ c *client.Client }

type credentialModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	Description    types.String `tfsdk:"description"`
	Allowlist      types.Set    `tfsdk:"allowlist"`
	CredentialJSON types.String `tfsdk:"credential_json"`
	AuthMethod     types.String `tfsdk:"auth_method"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *credentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_connector_credential"
}

func (r *credentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An API-connector credential: outbound auth Sigma uses when an `sigma_api_connector` calls an external HTTP endpoint. " +
			"Distinct from `sigma_api_credential` (which is OAuth for Sigma's own REST API). " +
			"The secret in `credential_json` is **write-only** — sent on create/update but never returned by the API, so refresh leaves it unchanged (no drift detection on that field).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, Description: "Display name of the credential."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
			"allowlist": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Hostname glob patterns the credential may be used against (e.g. `[\"*.example.com\"]`). Must be non-empty; use `[\"*\"]` to allow every host.",
			},
			"credential_json": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON-encoded `credential` object per Sigma's POST /v2/api-credentials oneOf (e.g. `{\"authMethod\":\"basic\",\"basic\":{\"username\":\"u\",\"password\":\"p\"}}`). Write-only: never returned by GET.",
			},
			"auth_method": schema.StringAttribute{Computed: true, Description: "Auth method derived from `credential_json` (e.g. `basic`)."},
			"created_at":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{Computed: true},
		},
	}
}

func (r *credentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan credentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, ok := r.buildBody(ctx, plan, resp.Diagnostics.AddAttributeError)
	if resp.Diagnostics.HasError() || !ok {
		return
	}
	var got apiCredential
	if err := r.c.Do(ctx, "POST", "/v2/api-credentials", body, &got); err != nil {
		resp.Diagnostics.AddError("Create API connector credential failed", err.Error())
		return
	}
	r.apply(&got, &plan) // preserves credential_json from plan
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state credentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got apiCredential
	err := r.c.Do(ctx, "GET", "/v2/api-credentials/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read API connector credential failed", err.Error())
		return
	}
	r.apply(&got, &state) // credential_json retained from state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state credentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, ok := r.buildBody(ctx, plan, resp.Diagnostics.AddAttributeError)
	if resp.Diagnostics.HasError() || !ok {
		return
	}
	var got apiCredential
	if err := r.c.Do(ctx, "PATCH", "/v2/api-credentials/"+client.EscapeID(state.ID.ValueString()), body, &got); err != nil {
		resp.Diagnostics.AddError("Update API connector credential failed", err.Error())
		return
	}
	r.apply(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state credentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/api-credentials/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete API connector credential failed", err.Error())
	}
}

func (r *credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildBody assembles the create/patch request from the plan. credential_json is
// parsed and sent verbatim as `credential`. Returns ok=false (with a diagnostic
// already appended) on invalid JSON.
func (r *credentialResource) buildBody(ctx context.Context, plan credentialModel, addErr func(path.Path, string, string)) (map[string]any, bool) {
	var allowlist []string
	plan.Allowlist.ElementsAs(ctx, &allowlist, false)

	var credential any
	if err := json.Unmarshal([]byte(plan.CredentialJSON.ValueString()), &credential); err != nil {
		addErr(path.Root("credential_json"), "Invalid JSON", err.Error())
		return nil, false
	}
	body := map[string]any{
		"name":       plan.Name.ValueString(),
		"allowlist":  allowlist,
		"credential": credential,
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body["description"] = plan.Description.ValueString()
	}
	return body, true
}

// apply copies server-returned fields into dst, leaving credential_json untouched
// (write-only). Allowlist/name/description are refreshed from the API.
func (r *credentialResource) apply(c *apiCredential, dst *credentialModel) {
	dst.ID = types.StringValue(c.APICredentialID)
	dst.Name = types.StringValue(c.Name)
	dst.Description = types.StringValue(c.Description)
	dst.AuthMethod = types.StringValue(c.AuthMethod)
	dst.Allowlist, _ = types.SetValueFrom(context.Background(), types.StringType, c.Allowlist)
	dst.CreatedAt = types.StringValue(c.CreatedAt.Format(tsLayout))
	dst.UpdatedAt = types.StringValue(c.UpdatedAt.Format(tsLayout))
}
