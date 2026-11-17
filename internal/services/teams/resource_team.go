// Package teams implements sigma_team and sigma_team_membership.
package teams

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
	_ resource.Resource                = (*teamResource)(nil)
	_ resource.ResourceWithConfigure   = (*teamResource)(nil)
	_ resource.ResourceWithImportState = (*teamResource)(nil)
)

func NewTeamResource() resource.Resource { return &teamResource{} }

type teamResource struct{ c *client.Client }

type teamModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Visibility  types.String `tfsdk:"visibility"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *teamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *teamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma team. Teams group members for permissioning and user-attribute assignment.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Sigma team UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, Description: "Team name."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Team description."},
			"visibility":  schema.StringAttribute{Optional: true, Computed: true, Description: "`public` or `private`. Defaults to `private`."},
			"created_at":  schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at":  schema.StringAttribute{Computed: true},
		},
	}
}

func (r *teamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createTeamReq{Name: plan.Name.ValueString()}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	if !plan.Visibility.IsNull() && !plan.Visibility.IsUnknown() {
		v := plan.Visibility.ValueString()
		body.Visibility = &v
	}
	var got team
	if err := r.c.Do(ctx, "POST", "/v2/teams", body, &got); err != nil {
		if client.IsConflict(err) {
			resp.Diagnostics.AddError("Team already exists",
				fmt.Sprintf("A team with name %q already exists. Run `terraform import sigma_team.<name> <teamId>`. %s", body.Name, err))
			return
		}
		resp.Diagnostics.AddError("Create team failed", err.Error())
		return
	}
	teamToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got team
	err := r.c.Do(ctx, "GET", "/v2/teams/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read team failed", err.Error())
		return
	}
	teamToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state teamModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := patchTeamReq{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		v := plan.Name.ValueString()
		body.Name = &v
	}
	if plan.Description.ValueString() != state.Description.ValueString() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	if plan.Visibility.ValueString() != state.Visibility.ValueString() {
		v := plan.Visibility.ValueString()
		body.Visibility = &v
	}
	var got team
	if err := r.c.Do(ctx, "PATCH", "/v2/teams/"+client.EscapeID(state.ID.ValueString()), body, &got); err != nil {
		resp.Diagnostics.AddError("Update team failed", err.Error())
		return
	}
	// PATCH returns the team; re-read only if it didn't.
	if got.TeamID == "" {
		if err := r.c.Do(ctx, "GET", "/v2/teams/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
			resp.Diagnostics.AddError("Re-read after update failed", err.Error())
			return
		}
	}
	teamToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/teams/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete team failed", err.Error())
	}
}

func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func teamToState(t *team, dst *teamModel) {
	dst.ID = types.StringValue(t.TeamID)
	dst.Name = types.StringValue(t.Name)
	if t.Description != nil {
		dst.Description = types.StringValue(*t.Description)
	} else {
		dst.Description = types.StringValue("")
	}
	dst.Visibility = types.StringValue(t.Visibility)
	dst.CreatedAt = types.StringValue(t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
}
