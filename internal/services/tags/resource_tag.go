// Package tags implements sigma_tag.
//
// Note: these are *version tags* — applied to specific workbook/report
// versions. Not Kubernetes-style key=value resource labels.
package tags

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*tagResource)(nil)
	_ resource.ResourceWithConfigure   = (*tagResource)(nil)
	_ resource.ResourceWithImportState = (*tagResource)(nil)
)

func NewResource() resource.Resource { return &tagResource{} }

type tagResource struct{ c *client.Client }

type tagModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Color       types.String `tfsdk:"color"`
	Description types.String `tfsdk:"description"`
	OwnerID     types.String `tfsdk:"owner_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type tag struct {
	VersionTagID string    `json:"versionTagId"`
	Name         string    `json:"name"`
	Color        string    `json:"color,omitempty"`
	Description  *string   `json:"description,omitempty"`
	OwnerID      string    `json:"ownerId,omitempty"`
	CreatedBy    string    `json:"createdBy,omitempty"`
	UpdatedBy    string    `json:"updatedBy,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type createTagReq struct {
	Name        string  `json:"name"`
	Color       string  `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
}

type patchTagReq struct {
	Description *string `json:"description,omitempty"`
}

func (r *tagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *tagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma version tag definition. Tags are applied to specific workbook/report versions via the Sigma API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Tag name. Must be unique. Immutable after create."},
			"color": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "One of `cyan`, `grass`, `violet`, `plum`, `amber`, `bronze`. Defaults to `cyan` if not set (the Sigma API rejects creates without a color). Immutable after create."},
			"description": schema.StringAttribute{Optional: true, Computed: true,
				Description: "Tag description. The only mutable attribute."},
			"owner_id":   schema.StringAttribute{Computed: true},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *tagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.Color.IsNull() || plan.Color.IsUnknown() || plan.Color.ValueString() == "" {
		plan.Color = types.StringValue("cyan")
	}
	body := createTagReq{Name: plan.Name.ValueString(), Color: plan.Color.ValueString()}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	defer client.InvalidateList(r.c, tagsKey)
	var got tag
	if err := r.c.Do(ctx, "POST", "/v2/tags", body, &got); err != nil {
		resp.Diagnostics.AddError("Create tag failed", err.Error())
		return
	}
	// Sigma's tag GET-by-id is eventually consistent (404 immediately after POST). Trust the
	// POST response for now; the next refresh's Read will fill audit fields. Populate
	// description/audit with values from plan or empty so all Computed attributes are known.
	plan.ID = types.StringValue(got.VersionTagID)
	plan.Name = types.StringValue(got.Name)
	if plan.Description.IsNull() || plan.Description.IsUnknown() {
		plan.Description = types.StringValue("")
	}
	plan.OwnerID = types.StringValue("")
	plan.CreatedAt = types.StringValue("")
	plan.UpdatedAt = types.StringValue("")
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Sigma's GET /v2/tags/{tagId} returns 404 for ~seconds after POST due to eventual
	// consistency. The list endpoint is reliable; use it — memoized per run so
	// refreshing N tags is one list, not N (writes invalidate the cache).
	wanted := state.ID.ValueString()
	all, err := client.CachedList(r.c, tagsKey, func() ([]tag, error) {
		return client.ListAllPaged[tag](ctx, r.c, "/v2/tags", nil)
	})
	if err != nil {
		resp.Diagnostics.AddError("List tags failed", err.Error())
		return
	}
	var found *tag
	for i := range all {
		if all[i].VersionTagID == wanted {
			found = &all[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	tagToState(found, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := patchTagReq{}
	if plan.Description.ValueString() != state.Description.ValueString() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	var patched tag
	if err := r.c.Do(ctx, "PATCH", "/v2/tags/"+client.EscapeID(state.ID.ValueString()), body, &patched); err != nil {
		resp.Diagnostics.AddError("Update tag failed", err.Error())
		return
	}
	client.InvalidateList(r.c, tagsKey)
	wanted := state.ID.ValueString()
	var found *tag
	var err error
	if patched.VersionTagID == wanted {
		found = &patched
	} else {
		// Re-read via list-and-find (GET-by-id is eventually consistent on Sigma's side).
		err = client.ForEachPaged[tag](ctx, r.c, "/v2/tags", nil, func(t tag) (bool, error) {
			if t.VersionTagID == wanted {
				found = &t
				return true, nil
			}
			return false, nil
		})
	}
	if err != nil || found == nil {
		// Fall back to plan values; next refresh will pick up server values.
		plan.ID = state.ID
		plan.Name = state.Name
		plan.Color = state.Color
		plan.OwnerID = state.OwnerID
		plan.CreatedAt = state.CreatedAt
		plan.UpdatedAt = state.UpdatedAt
	} else {
		tagToState(found, &plan)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, tagsKey)
	if err := r.c.Do(ctx, "DELETE", "/v2/tags/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete tag failed", err.Error())
	}
}

// tagsKey is the per-run CachedList key for the org's version-tag list.
const tagsKey = "tags"

func (r *tagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func tagToState(t *tag, dst *tagModel) {
	dst.ID = types.StringValue(t.VersionTagID)
	dst.Name = types.StringValue(t.Name)
	// Sigma's tag LIST endpoint omits `color` from the response — only the
	// GET-by-id endpoint includes it (and that's eventually consistent).
	// If the API returns no color but state already has one, preserve state's
	// value to avoid spurious "color forces replacement" diffs. color is
	// RequiresReplace and immutable, so this is safe.
	if t.Color != "" {
		dst.Color = types.StringValue(t.Color)
	}
	if t.Description != nil {
		dst.Description = types.StringValue(*t.Description)
	} else {
		dst.Description = types.StringValue("")
	}
	dst.OwnerID = types.StringValue(t.OwnerID)
	dst.CreatedAt = types.StringValue(t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
}
