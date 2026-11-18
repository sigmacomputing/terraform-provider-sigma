package jwks

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*keyResource)(nil)
	_ resource.ResourceWithConfigure   = (*keyResource)(nil)
	_ resource.ResourceWithImportState = (*keyResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*keyResource)(nil)
)

func NewKeyResource() resource.Resource { return &keyResource{} }

type keyResource struct{ c *client.Client }

type keyModel struct {
	ID           types.String `tfsdk:"id"`
	JwkSetID     types.String `tfsdk:"jwk_set_id"`
	KeyID        types.String `tfsdk:"key_id"`
	ValidForDays types.Int64  `tfsdk:"valid_for_days"`
	Active       types.Bool   `tfsdk:"active"`
	PublicKey    types.String `tfsdk:"public_key"`
	Algorithm    types.String `tfsdk:"algorithm"`
	ExpiresAt    types.String `tfsdk:"expires_at"`
}

func (r *keyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jwk_set_key"
}

func (r *keyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An additional key in a JWK set, for rotation. Sigma creates it inactive; set `active = true` to make it the set's signing key (the previous key stays, inactive, until deleted). A set holds at most two keys. There is no deactivate call: a key stops being active only when another key is activated, so after rotating again set the superseded key's `active` to `false` (or remove it), or the two keys will take the active role from each other on every apply. The active key can't be deleted, so destroying it only removes it from state, and `-replace` on an active key fails (the set would need a third key). Composite ID `<jwk_set_id>:<key_id>`; import the set's first key this way to delete it after rotating.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"jwk_set_id": schema.StringAttribute{Required: true, Description: "JWK set the key belongs to.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"key_id": schema.StringAttribute{Computed: true, Description: "JSON Web Key ID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"valid_for_days": schema.Int64Attribute{Optional: true,
				Description: "Certificate lifetime in days (Sigma defaults to 365). Used only at creation; changing it later has no effect."},
			"active": schema.BoolAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether this key signs for the set. Only `false` → `true` can be applied; activate a different key to deactivate this one."},
			"public_key": schema.StringAttribute{Computed: true, Description: "PEM X.509 certificate. Never includes private key material.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"algorithm": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"expires_at": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *keyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// fillKey copies a listed key onto the model; IDs come from known values, not the response.
func fillKey(m *keyModel, setID, keyID string, k keyResp) {
	m.ID = types.StringValue(setID + ":" + keyID)
	m.JwkSetID = types.StringValue(setID)
	m.KeyID = types.StringValue(keyID)
	m.Active = types.BoolValue(k.Status == "active")
	m.PublicKey = types.StringValue(k.PublicKey)
	m.Algorithm = types.StringValue(k.Algorithm)
	m.ExpiresAt = types.StringValue(k.ExpiresAt)
}

// findKey looks a key up in its set's list; found is false if the key or set is gone.
// Reads pass cached=true; writes need the live list.
func (r *keyResource) findKey(ctx context.Context, setID, keyID string, cached bool) (keyResp, bool, error) {
	list := listKeys
	if cached {
		list = cachedKeys
	}
	keys, err := list(ctx, r.c, setID)
	if err != nil && !client.IsNotFound(err) {
		return keyResp{}, false, err
	}
	for _, k := range keys {
		if k.JSONWebKeyID == keyID {
			return k, true, nil
		}
	}
	return keyResp{}, false, nil
}

// activateAndRead activates a key, then re-reads it from the list rather than trusting the activate body.
func (r *keyResource) activateAndRead(ctx context.Context, m *keyModel, setID, keyID string) error {
	defer invalidateKeys(r.c, setID)
	if err := r.c.Do(ctx, "POST", keyPath(setID, keyID)+":activate", struct{}{}, nil); err != nil {
		return err
	}
	k, found, err := r.findKey(ctx, setID, keyID, false)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("key %s not found in JWK set %s after activation", keyID, setID)
	}
	fillKey(m, setID, keyID, k)
	return nil
}

func (r *keyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var cfgActive, stateActive types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("active"), &cfgActive)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("active"), &stateActive)...)
	if resp.Diagnostics.HasError() || cfgActive.IsNull() || cfgActive.IsUnknown() {
		return
	}
	switch {
	case !cfgActive.ValueBool() && stateActive.ValueBool():
		resp.Diagnostics.AddAttributeError(path.Root("active"), "Can't deactivate a JSON Web Key",
			"Sigma has no deactivate call. Activate another key in the set instead; this key then becomes inactive.")
	case cfgActive.ValueBool() && !stateActive.ValueBool():
		resp.Diagnostics.AddAttributeWarning(path.Root("active"), "Activating this key deactivates the set's current key",
			"If another sigma_jwk_set_key in this set also has active = true, the two will take the active role from each other on every apply. Set the superseded key's active to false or remove the attribute.")
	}
}

func (r *keyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan keyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	setID := plan.JwkSetID.ValueString()
	var body validForReq
	if !plan.ValidForDays.IsNull() && !plan.ValidForDays.IsUnknown() {
		v := plan.ValidForDays.ValueInt64()
		body.ValidForDays = &v
	}
	var got keyResp
	defer invalidateKeys(r.c, setID)
	if err := r.c.Do(ctx, "POST", keysPath(setID), body, &got); err != nil {
		resp.Diagnostics.AddError("Create JSON Web Key failed", err.Error())
		return
	}
	if got.JSONWebKeyID == "" {
		resp.Diagnostics.AddError("Create JSON Web Key failed", "Sigma returned no jsonWebKeyId")
		return
	}
	wantActive := !plan.Active.IsNull() && !plan.Active.IsUnknown() && plan.Active.ValueBool()
	fillKey(&plan, setID, got.JSONWebKeyID, got)
	if wantActive {
		if err := r.activateAndRead(ctx, &plan, setID, got.JSONWebKeyID); err != nil {
			// Save the inactive key so it isn't orphaned; the next apply retries activation.
			resp.Diagnostics.AddError("Activate JSON Web Key failed", err.Error())
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *keyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state keyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	setID, keyID, ok := splitID(state.ID.ValueString())
	if !ok {
		resp.Diagnostics.AddError("Invalid state ID", state.ID.ValueString())
		return
	}
	k, found, err := r.findKey(ctx, setID, keyID, true)
	if err != nil {
		resp.Diagnostics.AddError("Read JSON Web Key failed", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	fillKey(&state, setID, keyID, k)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *keyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state keyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	setID, keyID, _ := splitID(state.ID.ValueString())
	switch {
	case plan.Active.ValueBool() && !state.Active.ValueBool():
		if err := r.activateAndRead(ctx, &plan, setID, keyID); err != nil {
			resp.Diagnostics.AddError("Activate JSON Web Key failed", err.Error())
			return
		}
	case !plan.Active.ValueBool() && state.Active.ValueBool():
		// ModifyPlan rejects this at plan time; kept as a guard.
		resp.Diagnostics.AddError("Can't deactivate a JSON Web Key",
			"Sigma has no deactivate call. Activate another key in the set instead; this key then becomes inactive.")
		return
	default:
		plan.Active = state.Active
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *keyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state keyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	setID, keyID, _ := splitID(state.ID.ValueString())
	defer invalidateKeys(r.c, setID)
	k, found, err := r.findKey(ctx, setID, keyID, false)
	if err != nil {
		resp.Diagnostics.AddError("Read JSON Web Key failed", err.Error())
		return
	}
	if !found {
		return
	}
	// Sigma won't delete the active key; leave it to be removed with its set.
	if k.Status == "active" {
		resp.Diagnostics.AddWarning("Active JSON Web Key left in place",
			"Sigma can't delete a set's active key, so it was removed from Terraform state only. It is deleted when its sigma_jwk_set is destroyed; to remove it sooner, activate another key and delete this one.")
		return
	}
	if err := r.c.Do(ctx, "DELETE", keyPath(setID, keyID), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete JSON Web Key failed", err.Error())
	}
}

func (r *keyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	setID, _, ok := splitID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<jwk_set_id>:<key_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("jwk_set_id"), setID)...)
}

func splitID(id string) (setID, keyID string, ok bool) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
