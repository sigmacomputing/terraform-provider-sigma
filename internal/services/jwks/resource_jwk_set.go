package jwks

import (
	"context"
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
	_ resource.Resource                = (*setResource)(nil)
	_ resource.ResourceWithConfigure   = (*setResource)(nil)
	_ resource.ResourceWithImportState = (*setResource)(nil)
)

func NewSetResource() resource.Resource { return &setResource{} }

type setResource struct{ c *client.Client }

type setModel struct {
	ID           types.String `tfsdk:"id"`
	ValidForDays types.Int64  `tfsdk:"valid_for_days"`
	ActiveKeyID  types.String `tfsdk:"active_key_id"`
	PublicKey    types.String `tfsdk:"public_key"`
	Algorithm    types.String `tfsdk:"algorithm"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
}

func (r *setResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jwk_set"
}

func (r *setResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A JWK set whose active key signs JWT bearer assertions for connections configured with `useJwt` (reference it as `jwkSetId` in `sigma_connection.details_json`). Sigma generates the first key as active when the set is created; register `public_key` with your identity provider. Rotate with sigma_jwk_set_key. The `active_key_id`, `public_key`, `algorithm` and `expires_at` attributes follow whichever key is active, so after a rotation they change on the next refresh, not in the same apply; during a rotation, register each key's own `public_key` with the identity provider rather than this moving value. Destroying the set deletes all its keys and breaks any connection still using it; Sigma can't check for that.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "JWK set ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"valid_for_days": schema.Int64Attribute{Optional: true,
				Description: "Lifetime in days of the first key's certificate (Sigma defaults to 365). Used only at creation; changing it later has no effect."},
			"active_key_id": schema.StringAttribute{Computed: true, Description: "ID of the key currently signing for this set.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"public_key": schema.StringAttribute{Computed: true, Description: "PEM X.509 certificate of the active key. Never includes private key material.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"algorithm": schema.StringAttribute{Computed: true, Description: "Signing algorithm of the active key (`RS256` or `ES256`).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"expires_at": schema.StringAttribute{Computed: true, Description: "Expiry of the active key's certificate.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *setResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// refresh fills the active-key fields; found is false once the set has no keys.
func (r *setResource) refresh(ctx context.Context, m *setModel) (found bool, err error) {
	keys, err := cachedKeys(ctx, r.c, m.ID.ValueString())
	if err != nil || len(keys) == 0 {
		return false, err
	}
	m.ActiveKeyID, m.PublicKey, m.Algorithm, m.ExpiresAt = types.StringNull(), types.StringNull(), types.StringNull(), types.StringNull()
	for _, k := range keys {
		if k.Status == "active" {
			m.ActiveKeyID = types.StringValue(k.JSONWebKeyID)
			m.PublicKey = types.StringValue(k.PublicKey)
			m.Algorithm = types.StringValue(k.Algorithm)
			m.ExpiresAt = types.StringValue(k.ExpiresAt)
		}
	}
	return true, nil
}

func (r *setResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan setModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var body validForReq
	if !plan.ValidForDays.IsNull() && !plan.ValidForDays.IsUnknown() {
		v := plan.ValidForDays.ValueInt64()
		body.ValidForDays = &v
	}
	var got keyResp
	if err := r.c.Do(ctx, "POST", "/v2/jwkSets", body, &got); err != nil {
		resp.Diagnostics.AddError("Create JWK set failed", err.Error())
		return
	}
	plan.ID = types.StringValue(got.JwkSetID)
	plan.ActiveKeyID = types.StringValue(got.JSONWebKeyID)
	plan.PublicKey = types.StringValue(got.PublicKey)
	plan.Algorithm = types.StringValue(got.Algorithm)
	plan.ExpiresAt = types.StringValue(got.ExpiresAt)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *setResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state setModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.refresh(ctx, &state)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Read JWK set failed", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *setResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Only valid_for_days can change, and it's creation-only: no API call.
	var plan, state setModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ValidForDays = plan.ValidForDays
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *setResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state setModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer invalidateKeys(r.c, state.ID.ValueString())
	if err := r.c.Do(ctx, "DELETE", setPath(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete JWK set failed", err.Error())
	}
}

func (r *setResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
