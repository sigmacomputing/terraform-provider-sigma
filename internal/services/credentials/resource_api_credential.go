// Package credentials implements sigma_api_credential.
//
// Sigma's POST /v2/credentials returns the secret EXACTLY ONCE at create time
// — there is no way to retrieve it later. The provider stores it in state
// (Sensitive). On state loss, the secret is gone; users must rotate.
package credentials

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*apiCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*apiCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*apiCredentialResource)(nil)
)

func NewResource() resource.Resource { return &apiCredentialResource{} }

type apiCredentialResource struct{ c *client.Client }

type apiCredentialModel struct {
	ID           types.String `tfsdk:"id"`
	OwnerID      types.String `tfsdk:"owner_id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	Scopes       types.Set    `tfsdk:"scopes"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

type createCredReq struct {
	OwnerID     string   `json:"ownerId"`
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
}

type credResp struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

func (r *apiCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_credential"
}

func (r *apiCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `Creates an OAuth2 client_id / client_secret pair for the Sigma REST API.

**⚠ Use an encrypted Terraform state backend.** This resource stores ` + "`client_secret`" + ` in state because Sigma's API returns the secret only once at create time and provides no GET endpoint to retrieve it later. Local-disk state is unsuitable for this resource — use S3 + KMS, Terraform Cloud / Enterprise, HashiCorp Vault, or another encrypted remote backend. Losing or exposing the state means rotating the credential.

Any change to inputs forces replacement (Sigma has no PATCH for credentials).`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Equal to client_id."},
			"owner_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Member UUID that will own the credential."},
			"name": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"description": schema.StringAttribute{Optional: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"scopes": schema.SetAttribute{Optional: true, Computed: true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Set{setplanmodifier.RequiresReplace()},
				Description:   "One or more of `api`, `embed`. Defaults to `[api]`. Changing this forces replacement (rotates the credential).",
			},
			"client_id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"client_secret": schema.StringAttribute{Computed: true, Sensitive: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Returned ONCE at create. Never re-readable. Surfaced to state and may be referenced via `nonsensitive(...)` only when intentional."},
		},
	}
}

func (r *apiCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *apiCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan apiCredentialModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createCredReq{
		OwnerID: plan.OwnerID.ValueString(),
		Name:    plan.Name.ValueString(),
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	if !plan.Scopes.IsNull() && !plan.Scopes.IsUnknown() {
		var scopes []string
		resp.Diagnostics.Append(plan.Scopes.ElementsAs(ctx, &scopes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.Scopes = scopes
	}

	var got credResp
	if err := r.c.Do(ctx, "POST", "/v2/credentials", body, &got); err != nil {
		resp.Diagnostics.AddError("Create API credential failed", err.Error())
		return
	}
	plan.ID = types.StringValue(got.ClientID)
	plan.ClientID = types.StringValue(got.ClientID)
	plan.ClientSecret = types.StringValue(got.ClientSecret)
	if plan.Scopes.IsNull() || plan.Scopes.IsUnknown() {
		// Default scopes when unspecified.
		plan.Scopes, _ = types.SetValueFrom(ctx, types.StringType, []string{"api"})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *apiCredentialResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// Sigma has no GET endpoint for credentials. We trust state — drift is undetectable.
	// If the credential is revoked server-side, the next Sigma API call using it will 401.
}

func (r *apiCredentialResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All inputs are RequiresReplace.
}

func (r *apiCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state apiCredentialModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/credentials/"+client.EscapeID(state.ClientID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete API credential failed", err.Error())
	}
}

func (r *apiCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Importing only recovers client_id; client_secret cannot be re-fetched.
	resp.Diagnostics.AddWarning(
		"Imported API credential is missing the client secret",
		"Sigma does not expose `client_secret` after creation. The imported state will have an empty secret. Rotate the credential by tainting and recreating if you need a usable secret in state.",
	)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_secret"), "")...)
}
