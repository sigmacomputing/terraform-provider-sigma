// Package workspaces implements sigma_workspace and sigma_workspace_grant.
package workspaces

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
	_ resource.Resource                = (*workspaceResource)(nil)
	_ resource.ResourceWithConfigure   = (*workspaceResource)(nil)
	_ resource.ResourceWithImportState = (*workspaceResource)(nil)
)

func NewResource() resource.Resource { return &workspaceResource{} }

type workspaceResource struct{ c *client.Client }

type workspaceModel struct {
	ID             types.String `tfsdk:"id"`
	WorkspaceURLID types.String `tfsdk:"workspace_url_id"`
	Name           types.String `tfsdk:"name"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *workspaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace"
}

func (r *workspaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma workspace. Workspaces group content and have their own permission grants (4-permission scheme: view, explore, organize, edit).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Workspace UUID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"workspace_url_id": schema.StringAttribute{
				Computed:    true,
				Description: "Base62-encoded ID used in workspace URLs.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{Required: true, Description: "Workspace name."},
			"created_at": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *workspaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *workspaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan workspaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	noDup := true
	body := createWorkspaceReq{Name: plan.Name.ValueString(), NoDuplicates: &noDup}
	var got workspace
	if err := r.c.Do(ctx, "POST", "/v2/workspaces", body, &got); err != nil {
		resp.Diagnostics.AddError("Create workspace failed", err.Error())
		return
	}
	wsToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workspaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state workspaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got workspace
	err := r.c.Do(ctx, "GET", "/v2/workspaces/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	// A deleted workspace is archived; Sigma then returns 409 inode_archived on
	// GET (not 404). Treat both as "gone" so out-of-band deletion surfaces as drift.
	if client.IsNotFound(err) || client.IsArchived(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read workspace failed", err.Error())
		return
	}
	wsToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *workspaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state workspaceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := patchWorkspaceReq{Name: plan.Name.ValueString()}
	var got workspace
	if err := r.c.Do(ctx, "PATCH", "/v2/workspaces/"+client.EscapeID(state.ID.ValueString()), body, &got); err != nil {
		resp.Diagnostics.AddError("Update workspace failed", err.Error())
		return
	}
	wsToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *workspaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state workspaceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// An already-archived (or already-deleted) workspace is effectively gone:
	// tolerate 404 and 409 inode_archived so destroy is idempotent.
	if err := r.c.Do(ctx, "DELETE", "/v2/workspaces/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) && !client.IsArchived(err) {
		resp.Diagnostics.AddError("Delete workspace failed", err.Error())
	}
}

func (r *workspaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func wsToState(w *workspace, dst *workspaceModel) {
	dst.ID = types.StringValue(w.WorkspaceID)
	dst.WorkspaceURLID = types.StringValue(w.WorkspaceURLID)
	dst.Name = types.StringValue(w.Name)
	dst.CreatedAt = types.StringValue(w.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(w.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
}
