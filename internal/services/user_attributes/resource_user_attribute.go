// Package user_attributes implements sigma_user_attribute and the team/user
// assignment resources. Sigma's API has no PATCH for the user attribute
// itself — name/description/defaultValue changes trigger replace.
package user_attributes

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
	_ resource.Resource                = (*uaResource)(nil)
	_ resource.ResourceWithConfigure   = (*uaResource)(nil)
	_ resource.ResourceWithImportState = (*uaResource)(nil)
)

func NewResource() resource.Resource { return &uaResource{} }

type uaResource struct{ c *client.Client }

type uaModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	DefaultValue types.String `tfsdk:"default_value"`
	CreatedAt    types.String `tfsdk:"created_at"`
	UpdatedAt    types.String `tfsdk:"updated_at"`
}

func (r *uaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute"
}

func (r *uaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma user attribute. Sigma supports only string-typed attributes today; assign values to teams and users via `sigma_user_attribute_team_assignment` and `sigma_user_attribute_user_assignment`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "User attribute UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Attribute name. Used by Sigma when querying datasets and applying RLS rules.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Human-readable description.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"default_value": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				Description:   "Default string value applied when neither team nor user assignment matches. Marked Sensitive because UA values commonly carry tenant IDs, region scopes, or PII-adjacent identifiers used for row-level security.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"created_at": schema.StringAttribute{Computed: true},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *uaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *uaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan uaModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createUARequest{Name: plan.Name.ValueString()}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		v := plan.Description.ValueString()
		body.Description = &v
	}
	if !plan.DefaultValue.IsNull() && !plan.DefaultValue.IsUnknown() {
		body.DefaultValue = &defaultValue{Val: plan.DefaultValue.ValueString(), Type: "string"}
	}
	var got userAttribute
	if err := r.c.Do(ctx, "POST", "/v2/user-attributes", body, &got); err != nil {
		resp.Diagnostics.AddError("Create user attribute failed", err.Error())
		return
	}
	uaToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *uaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state uaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got userAttribute
	err := r.c.Do(ctx, "GET", "/v2/user-attributes/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read user attribute failed", err.Error())
		return
	}
	uaToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *uaResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All mutable attrs are RequiresReplace.
}

func (r *uaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state uaModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/user-attributes/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete user attribute failed", err.Error())
	}
}

func (r *uaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func uaToState(u *userAttribute, dst *uaModel) {
	dst.ID = types.StringValue(u.UserAttributeID)
	dst.Name = types.StringValue(u.Name)
	if u.Description != nil {
		dst.Description = types.StringValue(*u.Description)
	} else {
		dst.Description = types.StringValue("")
	}
	if u.DefaultValue != nil {
		dst.DefaultValue = types.StringValue(u.DefaultValue.Val)
	} else {
		dst.DefaultValue = types.StringNull()
	}
	dst.CreatedAt = types.StringValue(u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(u.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
}
