// Package sourceswap implements sigma_source_swap_policy.
package sourceswap

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*sourceSwapPolicyResource)(nil)
	_ resource.ResourceWithConfigure   = (*sourceSwapPolicyResource)(nil)
	_ resource.ResourceWithImportState = (*sourceSwapPolicyResource)(nil)
)

func NewResource() resource.Resource { return &sourceSwapPolicyResource{} }

type sourceSwapPolicyResource struct{ c *client.Client }

type sourceSwapPolicyModel struct {
	ID               types.String `tfsdk:"id"`
	Type             types.String `tfsdk:"type"`
	Name             types.String `tfsdk:"name"`
	FromConnectionID types.String `tfsdk:"from_connection_id"`
	ConnectionSwaps  types.List   `tfsdk:"connection_swaps"`
}

// connectionSwapModel mirrors one entry of the API's `swaps.connectionSwaps[]`.
type connectionSwapModel struct {
	ToConnectionID types.String `tfsdk:"to_connection_id"`
	Path0          types.Object `tfsdk:"path0"`
	Path1          types.Object `tfsdk:"path1"`
	Path2          types.Object `tfsdk:"path2"`
}

// pathSwapModel is one path component within a connection swap. swap_type is
// the discriminator: `exact` (use catalog_object), `attribute` (use
// user_attribute_id), or `all` (pass through; no other fields).
type pathSwapModel struct {
	SwapType        types.String `tfsdk:"swap_type"`
	CatalogObject   types.String `tfsdk:"catalog_object"`
	UserAttributeID types.String `tfsdk:"user_attribute_id"`
}

var pathSwapAttrTypes = map[string]attr.Type{
	"swap_type":         types.StringType,
	"catalog_object":    types.StringType,
	"user_attribute_id": types.StringType,
}

var connectionSwapAttrTypes = map[string]attr.Type{
	"to_connection_id": types.StringType,
	"path0":            types.ObjectType{AttrTypes: pathSwapAttrTypes},
	"path1":            types.ObjectType{AttrTypes: pathSwapAttrTypes},
	"path2":            types.ObjectType{AttrTypes: pathSwapAttrTypes},
}

type sourceSwapPolicyResp struct {
	PolicyID         string         `json:"policyId"`
	Type             string         `json:"type"`
	Name             string         `json:"name"`
	FromConnectionID string         `json:"fromConnectionId"`
	Swaps            map[string]any `json:"swaps"`
}

type createSourceSwapReq struct {
	Type             string         `json:"type"`
	Name             string         `json:"name"`
	FromConnectionID string         `json:"fromConnectionId"`
	Swaps            map[string]any `json:"swaps"`
}

type patchSourceSwapReq struct {
	Type             *string        `json:"type,omitempty"`
	Name             *string        `json:"name,omitempty"`
	FromConnectionID *string        `json:"fromConnectionId,omitempty"`
	Swaps            map[string]any `json:"swaps,omitempty"`
}

func (r *sourceSwapPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_swap_policy"
}

func (r *sourceSwapPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	pathAttrs := map[string]schema.Attribute{
		"swap_type": schema.StringAttribute{Required: true,
			Description: "Resolution strategy for this path component. One of `exact` (literal — set `catalog_object`), `attribute` (resolve via `user_attribute_id`), or `all` (pass through; no other fields)."},
		"catalog_object":    schema.StringAttribute{Optional: true, Description: "Literal catalog name. Required when `swap_type = \"exact\"`."},
		"user_attribute_id": schema.StringAttribute{Optional: true, Description: "User attribute ID whose value supplies the path. Required when `swap_type = \"attribute\"`."},
	}
	resp.Schema = schema.Schema{
		Description: "A Sigma source swap policy. Defines rules for swapping connection paths, enabling environment promotion and multi-tenant content deployment. `type` is immutable after creation.",
		Attributes: map[string]schema.Attribute{
			"id":                 schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"type":               schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "Policy type. Currently only `\"connection\"` is supported."},
			"name":               schema.StringAttribute{Required: true, Description: "Display name of the policy."},
			"from_connection_id": schema.StringAttribute{Required: true, Description: "ID of the source connection whose paths will be swapped."},
			"connection_swaps": schema.ListNestedAttribute{
				Required:    true,
				Description: "Per-connection swap rules. Each entry maps a target connection ID and resolves three path components (`path0`, `path1`, `path2`) — typically database, schema, table.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"to_connection_id": schema.StringAttribute{Required: true, Description: "Target connection ID — paths matching this swap rule are rewritten to point at this connection."},
						"path0": schema.SingleNestedAttribute{
							Required:    true,
							Attributes:  pathAttrs,
							Description: "Resolution rule for the first path component (typically the database).",
						},
						"path1": schema.SingleNestedAttribute{
							Optional:    true,
							Attributes:  pathAttrs,
							Description: "Resolution rule for the second path component (typically the schema).",
						},
						"path2": schema.SingleNestedAttribute{
							Optional:    true,
							Attributes:  pathAttrs,
							Description: "Resolution rule for the third path component (typically the table).",
						},
					},
				},
			},
		},
	}
}

func (r *sourceSwapPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *sourceSwapPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sourceSwapPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	swaps, diags := buildAPISwaps(ctx, plan.ConnectionSwaps)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createSourceSwapReq{
		Type:             plan.Type.ValueString(),
		Name:             plan.Name.ValueString(),
		FromConnectionID: plan.FromConnectionID.ValueString(),
		Swaps:            swaps,
	}
	var created sourceSwapPolicyResp
	if err := r.c.Do(ctx, "POST", "/v2/sourceSwapPolicies", body, &created); err != nil {
		resp.Diagnostics.AddError("Create source swap policy failed", err.Error())
		return
	}
	// POST only returns policyId — read full state back.
	var got sourceSwapPolicyResp
	if err := r.c.Do(ctx, "GET", "/v2/sourceSwapPolicies/"+client.EscapeID(created.PolicyID), nil, &got); err != nil {
		resp.Diagnostics.AddError("Read source swap policy after create failed", err.Error())
		return
	}
	d := sspToState(ctx, &got, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sourceSwapPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sourceSwapPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got sourceSwapPolicyResp
	if err := r.c.Do(ctx, "GET", "/v2/sourceSwapPolicies/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read source swap policy failed", err.Error())
		return
	}
	d := sspToState(ctx, &got, &state)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sourceSwapPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state sourceSwapPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := patchSourceSwapReq{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		v := plan.Name.ValueString()
		patch.Name = &v
	}
	if plan.FromConnectionID.ValueString() != state.FromConnectionID.ValueString() {
		v := plan.FromConnectionID.ValueString()
		patch.FromConnectionID = &v
	}
	if !plan.ConnectionSwaps.Equal(state.ConnectionSwaps) {
		swaps, diags := buildAPISwaps(ctx, plan.ConnectionSwaps)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		patch.Swaps = swaps
	}
	if err := r.c.Do(ctx, "PATCH", "/v2/sourceSwapPolicies/"+client.EscapeID(state.ID.ValueString()), patch, nil); err != nil {
		resp.Diagnostics.AddError("Update source swap policy failed", err.Error())
		return
	}
	// PATCH does not echo the full body — read full state back.
	var got sourceSwapPolicyResp
	if err := r.c.Do(ctx, "GET", "/v2/sourceSwapPolicies/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		resp.Diagnostics.AddError("Read source swap policy after update failed", err.Error())
		return
	}
	d := sspToState(ctx, &got, &plan)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sourceSwapPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sourceSwapPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/sourceSwapPolicies/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete source swap policy failed", err.Error())
	}
}

func (r *sourceSwapPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildAPISwaps converts the typed connection_swaps list into the API's
// {connectionSwaps: [...]} JSON shape.
func buildAPISwaps(ctx context.Context, list types.List) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		diags.AddAttributeError(path.Root("connection_swaps"), "Missing connection_swaps", "connection_swaps must contain at least one entry")
		return nil, diags
	}
	var swaps []connectionSwapModel
	diags.Append(list.ElementsAs(ctx, &swaps, false)...)
	if diags.HasError() {
		return nil, diags
	}
	if len(swaps) == 0 {
		diags.AddAttributeError(path.Root("connection_swaps"), "Missing connection_swaps", "connection_swaps must contain at least one entry")
		return nil, diags
	}
	out := make([]map[string]any, 0, len(swaps))
	for i, s := range swaps {
		entry := map[string]any{
			"toConnectionId": s.ToConnectionID.ValueString(),
		}
		to := map[string]any{}
		for _, p := range []struct {
			key string
			obj types.Object
		}{
			{"path0", s.Path0},
			{"path1", s.Path1},
			{"path2", s.Path2},
		} {
			if p.obj.IsNull() || p.obj.IsUnknown() {
				continue
			}
			pathAPI, err := pathToAPI(ctx, p.obj)
			if err != "" {
				diags.AddAttributeError(path.Root("connection_swaps").AtListIndex(i).AtName(p.key), "Invalid path swap", err)
				return nil, diags
			}
			to[p.key] = pathAPI
		}
		entry["to"] = to
		out = append(out, entry)
	}
	return map[string]any{"connectionSwaps": out}, diags
}

// pathToAPI builds one path component object enforcing the swap_type rules.
func pathToAPI(ctx context.Context, obj types.Object) (map[string]any, string) {
	var p pathSwapModel
	d := obj.As(ctx, &p, basetypes.ObjectAsOptions{UnhandledNullAsEmpty: true, UnhandledUnknownAsEmpty: true})
	if d.HasError() {
		return nil, "failed to decode path object"
	}
	swapType := p.SwapType.ValueString()
	out := map[string]any{"swapType": swapType}
	switch swapType {
	case "exact":
		if p.CatalogObject.IsNull() || p.CatalogObject.ValueString() == "" {
			return nil, `swap_type = "exact" requires catalog_object to be set`
		}
		if !p.UserAttributeID.IsNull() && p.UserAttributeID.ValueString() != "" {
			return nil, `swap_type = "exact" must not have user_attribute_id set`
		}
		out["catalogObject"] = p.CatalogObject.ValueString()
	case "attribute":
		if p.UserAttributeID.IsNull() || p.UserAttributeID.ValueString() == "" {
			return nil, `swap_type = "attribute" requires user_attribute_id to be set`
		}
		if !p.CatalogObject.IsNull() && p.CatalogObject.ValueString() != "" {
			return nil, `swap_type = "attribute" must not have catalog_object set`
		}
		out["userAttributeId"] = p.UserAttributeID.ValueString()
	case "all":
		if !p.CatalogObject.IsNull() && p.CatalogObject.ValueString() != "" {
			return nil, `swap_type = "all" must not have catalog_object set`
		}
		if !p.UserAttributeID.IsNull() && p.UserAttributeID.ValueString() != "" {
			return nil, `swap_type = "all" must not have user_attribute_id set`
		}
	default:
		return nil, fmt.Sprintf(`unknown swap_type %q (expected "exact", "attribute", or "all")`, swapType)
	}
	return out, ""
}

func sspToState(ctx context.Context, s *sourceSwapPolicyResp, dst *sourceSwapPolicyModel) diag.Diagnostics {
	dst.ID = types.StringValue(s.PolicyID)
	dst.Type = types.StringValue(s.Type)
	dst.Name = types.StringValue(s.Name)
	dst.FromConnectionID = types.StringValue(s.FromConnectionID)
	swaps, diags := apiSwapsToList(ctx, s.Swaps)
	if diags.HasError() {
		return diags
	}
	dst.ConnectionSwaps = swaps
	return diags
}

// apiSwapsToList converts the API's `{connectionSwaps: [...]}` map back into
// a typed types.List for state.
func apiSwapsToList(ctx context.Context, swaps map[string]any) (types.List, diag.Diagnostics) {
	listType := types.ListType{ElemType: types.ObjectType{AttrTypes: connectionSwapAttrTypes}}
	if swaps == nil {
		return types.ListNull(listType.ElemType), nil
	}
	rawSwaps, ok := swaps["connectionSwaps"].([]any)
	if !ok {
		return types.ListNull(listType.ElemType), nil
	}
	out := make([]connectionSwapModel, 0, len(rawSwaps))
	for _, raw := range rawSwaps {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		toConnID, _ := entry["toConnectionId"].(string)
		toMap, _ := entry["to"].(map[string]any)
		swap := connectionSwapModel{
			ToConnectionID: types.StringValue(toConnID),
			Path0:          apiPathToObject(toMap, "path0"),
			Path1:          apiPathToObject(toMap, "path1"),
			Path2:          apiPathToObject(toMap, "path2"),
		}
		out = append(out, swap)
	}
	val, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: connectionSwapAttrTypes}, out)
	return val, diags
}

func apiPathToObject(toMap map[string]any, key string) types.Object {
	objType := types.ObjectType{AttrTypes: pathSwapAttrTypes}
	if toMap == nil {
		return types.ObjectNull(pathSwapAttrTypes)
	}
	raw, ok := toMap[key].(map[string]any)
	if !ok {
		return types.ObjectNull(pathSwapAttrTypes)
	}
	swapType, _ := raw["swapType"].(string)
	attrs := map[string]attr.Value{
		"swap_type":         types.StringValue(swapType),
		"catalog_object":    types.StringNull(),
		"user_attribute_id": types.StringNull(),
	}
	if v, ok := raw["catalogObject"].(string); ok && v != "" {
		attrs["catalog_object"] = types.StringValue(v)
	}
	if v, ok := raw["userAttributeId"].(string); ok && v != "" {
		attrs["user_attribute_id"] = types.StringValue(v)
	}
	obj, _ := types.ObjectValue(objType.AttrTypes, attrs)
	return obj
}
