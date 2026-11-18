// Package folders implements sigma_folder via the /v2/files endpoint.
package folders

import (
	"context"
	"fmt"
	"time"

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
	_ resource.Resource                = (*folderResource)(nil)
	_ resource.ResourceWithConfigure   = (*folderResource)(nil)
	_ resource.ResourceWithImportState = (*folderResource)(nil)
)

func NewResource() resource.Resource { return &folderResource{} }

type folderResource struct{ c *client.Client }

type folderModel struct {
	ID          types.String `tfsdk:"id"`
	URLId       types.String `tfsdk:"url_id"`
	Name        types.String `tfsdk:"name"`
	ParentID    types.String `tfsdk:"parent_id"`
	Description types.String `tfsdk:"description"`
	OwnerID     types.String `tfsdk:"owner_id"`
	Path        types.String `tfsdk:"path"`
	IsArchived  types.Bool   `tfsdk:"is_archived"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

type folderFile struct {
	ID          string    `json:"id"`
	URLId       string    `json:"urlId"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	ParentID    string    `json:"parentId"`
	Path        string    `json:"path"`
	OwnerID     string    `json:"ownerId"`
	IsArchived  bool      `json:"isArchived"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	Description *string   `json:"description,omitempty"`
}

type createFolderReq struct {
	Type        string  `json:"type"`
	Name        string  `json:"name"`
	ParentID    *string `json:"parentId,omitempty"`
	Description *string `json:"description,omitempty"`
	OwnerID     *string `json:"ownerId,omitempty"`
}

type patchFolderReq struct {
	Name        *string `json:"name,omitempty"`
	ParentID    *string `json:"parentId,omitempty"`
	Description *string `json:"description,omitempty"`
	OwnerID     *string `json:"ownerId,omitempty"`
}

func (r *folderResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_folder"
}

func (r *folderResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A folder in Sigma's file system. Folders can be nested inside workspaces or other folders via `parent_id`. Deleting a folder archives it — content is preserved and can be restored.",
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"url_id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":   schema.StringAttribute{Required: true},
			"parent_id": schema.StringAttribute{Optional: true, Computed: true,
				Description:   "ID of the parent folder or workspace. Omit to create in the org root.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"description": schema.StringAttribute{Optional: true, Computed: true},
			"owner_id":    schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"path":        schema.StringAttribute{Computed: true},
			"is_archived": schema.BoolAttribute{Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
			"created_at":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{Computed: true},
		},
	}
}

func (r *folderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *folderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan folderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createFolderReq{Type: "folder", Name: plan.Name.ValueString()}
	if !plan.ParentID.IsNull() && !plan.ParentID.IsUnknown() {
		v := plan.ParentID.ValueString()
		body.ParentID = &v
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	if !plan.OwnerID.IsNull() && !plan.OwnerID.IsUnknown() {
		v := plan.OwnerID.ValueString()
		body.OwnerID = &v
	}
	var got folderFile
	if err := r.c.Do(ctx, "POST", "/v2/files", body, &got); err != nil {
		resp.Diagnostics.AddError("Create folder failed", err.Error())
		return
	}
	folderToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *folderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state folderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got folderFile
	if err := r.c.Do(ctx, "GET", "/v2/files/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		if client.IsNotFound(err) || client.IsArchived(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read folder failed", err.Error())
		return
	}
	// A deleted folder is archived, but GET /v2/files/{id} still returns 200 with
	// isArchived=true. Treat that as "gone" so out-of-band deletion surfaces as drift.
	if got.IsArchived {
		resp.State.RemoveResource(ctx)
		return
	}
	folderToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *folderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state folderModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := patchFolderReq{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		v := plan.Name.ValueString()
		patch.Name = &v
	}
	if plan.ParentID.ValueString() != state.ParentID.ValueString() {
		v := plan.ParentID.ValueString()
		patch.ParentID = &v
	}
	if plan.Description.ValueString() != state.Description.ValueString() {
		v := plan.Description.ValueString()
		patch.Description = &v
	}
	if plan.OwnerID.ValueString() != state.OwnerID.ValueString() {
		v := plan.OwnerID.ValueString()
		patch.OwnerID = &v
	}
	var got folderFile
	if err := r.c.Do(ctx, "PATCH", "/v2/files/"+client.EscapeID(state.ID.ValueString()), patch, &got); err != nil {
		resp.Diagnostics.AddError("Update folder failed", err.Error())
		return
	}
	folderToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete archives the folder. Content is preserved and can be restored via the Sigma UI.
func (r *folderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state folderModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/files/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete folder failed", err.Error())
	}
}

func (r *folderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func folderToState(f *folderFile, dst *folderModel) {
	dst.ID = types.StringValue(f.ID)
	dst.URLId = types.StringValue(f.URLId)
	dst.Name = types.StringValue(f.Name)
	dst.ParentID = types.StringValue(f.ParentID)
	dst.Path = types.StringValue(f.Path)
	dst.OwnerID = types.StringValue(f.OwnerID)
	dst.IsArchived = types.BoolValue(f.IsArchived)
	dst.CreatedAt = types.StringValue(f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	if f.Description != nil {
		dst.Description = types.StringValue(*f.Description)
	} else {
		dst.Description = types.StringNull()
	}
}
