// Package allowed_ips implements sigma_allowed_ip + sigma_allowed_ips.
//
// Sigma's IP allowlist API is batch-only: POST :batchCreate for additions,
// POST :batchDelete for removals. We expose a single-entry resource here
// (one TF resource = one IP allowlist entry) by sending a one-element
// batch on Create and a one-element batch on Delete. Reads list all
// entries and find by ID. This matches Terraform's per-resource model
// while still using the batch endpoints under the hood.
package allowed_ips

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
	_ resource.Resource                = (*allowedIPResource)(nil)
	_ resource.ResourceWithConfigure   = (*allowedIPResource)(nil)
	_ resource.ResourceWithImportState = (*allowedIPResource)(nil)
)

func NewResource() resource.Resource { return &allowedIPResource{} }

type allowedIPResource struct{ c *client.Client }

type allowedIPModel struct {
	ID          types.String `tfsdk:"id"`
	IP          types.String `tfsdk:"ip"`
	Scope       types.String `tfsdk:"scope"`
	Description types.String `tfsdk:"description"`
}

type allowedIPEntry struct {
	IPAllowlistEntryID string `json:"ipAllowlistEntryId,omitempty"`
	IP                 string `json:"ip"`
	Scope              string `json:"scope"`
	Description        string `json:"description,omitempty"`
}

type batchCreateReq struct {
	Entries []allowedIPEntry `json:"entries"`
}

type batchCreateResp struct {
	Entries []allowedIPEntry `json:"entries"`
}

type batchDeleteReq struct {
	IPAllowlistEntryIDs []string `json:"ipAllowlistEntryIds"`
}

func (r *allowedIPResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allowed_ip"
}

func (r *allowedIPResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An entry in the org's IP allowlist (Sigma beta API). One TF resource manages one entry; the underlying API is batch-only, so Create sends a one-element batchCreate and Destroy sends a one-element batchDelete.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Sigma-assigned `ipAllowlistEntryId`.",
			},
			"ip": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "IPv4 / IPv6 address or CIDR range.",
			},
			"scope": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "One of `public-api`, `ui`, or `both` — which surface the allowlist entry applies to.",
			},
			"description": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Human-readable description of the entry.",
			},
		},
	}
}

func (r *allowedIPResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *allowedIPResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan allowedIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := batchCreateReq{
		Entries: []allowedIPEntry{{
			IP:          plan.IP.ValueString(),
			Scope:       plan.Scope.ValueString(),
			Description: plan.Description.ValueString(),
		}},
	}
	defer client.InvalidateList(r.c, allowedIPsKey)
	var got batchCreateResp
	if err := r.c.Do(ctx, "POST", "/v3alpha/allowedIps:batchCreate", body, &got); err != nil {
		resp.Diagnostics.AddError("Create allowed IP failed", err.Error())
		return
	}
	if len(got.Entries) == 0 {
		resp.Diagnostics.AddError("Create allowed IP failed", "API returned empty entries list")
		return
	}
	plan.ID = types.StringValue(got.Entries[0].IPAllowlistEntryID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *allowedIPResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state allowedIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	// The v3alpha allowlist API is batch-only with no GET-by-id, so reads list
	// all entries. Memoize the list per run so refreshing N entries is one list,
	// not N (writes invalidate the cache).
	all, err := client.CachedList(r.c, allowedIPsKey, func() ([]allowedIPEntry, error) {
		return client.ListAllPageToken[allowedIPEntry](ctx, r.c, "/v3alpha/allowedIps", client.PageSize(client.MaxPageSize))
	})
	if err != nil {
		resp.Diagnostics.AddError("List allowed IPs failed", err.Error())
		return
	}
	var found *allowedIPEntry
	for i := range all {
		if all[i].IPAllowlistEntryID == id {
			found = &all[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.IP = types.StringValue(found.IP)
	state.Scope = types.StringValue(found.Scope)
	if found.Description != "" {
		state.Description = types.StringValue(found.Description)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *allowedIPResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace; Update is unreachable.
}

func (r *allowedIPResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state allowedIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, allowedIPsKey)
	body := batchDeleteReq{IPAllowlistEntryIDs: []string{state.ID.ValueString()}}
	if err := r.c.Do(ctx, "POST", "/v3alpha/allowedIps:batchDelete", body, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete allowed IP failed", err.Error())
	}
}

// allowedIPsKey is the per-run CachedList key for the org's IP allowlist.
const allowedIPsKey = "allowed-ips"

func (r *allowedIPResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
