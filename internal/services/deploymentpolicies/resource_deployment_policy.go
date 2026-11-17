package deploymentpolicies

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*deploymentPolicyResource)(nil)
	_ resource.ResourceWithConfigure   = (*deploymentPolicyResource)(nil)
	_ resource.ResourceWithImportState = (*deploymentPolicyResource)(nil)
)

func NewResource() resource.Resource { return &deploymentPolicyResource{} }

type deploymentPolicyResource struct{ c *client.Client }

type deploymentPolicyModel struct {
	ID                       types.String `tfsdk:"id"`
	Name                     types.String `tfsdk:"name"`
	NameInTenant             types.String `tfsdk:"name_in_tenant"`
	VersionTagID             types.String `tfsdk:"version_tag_id"`
	SourceSwapPolicies       types.List   `tfsdk:"source_swap_policies"`
	CopyInputTableData       types.Bool   `tfsdk:"copy_input_table_data"`
	UseDependenciesWorkspace types.Bool   `tfsdk:"use_dependencies_workspace"`
}

func (r *deploymentPolicyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_policy"
}

func (r *deploymentPolicyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma deployment policy: controls how content is promoted into tenant organizations, including which source-swap policies apply and whether input table data is copied. Use sigma_deployment_policy_file and sigma_deployment_policy_tenant to attach files and target tenants.",
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name": schema.StringAttribute{Required: true, Description: "Display name of the deployment policy."},
			"name_in_tenant": schema.StringAttribute{Optional: true, Computed: true,
				Description: "Name to use for the workspace created by the deployment policy in the receiving tenant. Defaults to `name` when omitted."},
			"version_tag_id": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Version tag ID pinned at creation. Not patchable — Sigma's update endpoint does not accept this field, so changing it forces replacement.",
			},
			"source_swap_policies": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "IDs of sigma_source_swap_policy resources used to swap connections when deploying documents.",
			},
			"copy_input_table_data": schema.BoolAttribute{Optional: true, Computed: true,
				Description: "Whether input table data is copied to receiving tenant organizations when deploying documents. Only applies to input tables editable in draft mode."},
			"use_dependencies_workspace": schema.BoolAttribute{Optional: true, Computed: true,
				Description: "Whether implicit dependencies are placed in a different workspace, separate from the deployment workspace."},
		},
	}
}

func (r *deploymentPolicyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *deploymentPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createDeploymentPolicyReq{Name: plan.Name.ValueString()}
	if !plan.NameInTenant.IsNull() && !plan.NameInTenant.IsUnknown() {
		v := plan.NameInTenant.ValueString()
		body.NameInTenant = &v
	}
	if !plan.VersionTagID.IsNull() && !plan.VersionTagID.IsUnknown() {
		v := plan.VersionTagID.ValueString()
		body.VersionTagID = &v
	}
	if !plan.CopyInputTableData.IsNull() && !plan.CopyInputTableData.IsUnknown() {
		v := plan.CopyInputTableData.ValueBool()
		body.CopyInputTableData = &v
	}
	if !plan.UseDependenciesWorkspace.IsNull() && !plan.UseDependenciesWorkspace.IsUnknown() {
		v := plan.UseDependenciesWorkspace.ValueBool()
		body.UseDependenciesWorkspace = &v
	}
	if !plan.SourceSwapPolicies.IsNull() && !plan.SourceSwapPolicies.IsUnknown() {
		var ids []string
		resp.Diagnostics.Append(plan.SourceSwapPolicies.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		body.SourceSwapPolicies = ids
	}

	var created createDeploymentPolicyResp
	if err := r.c.Do(ctx, "POST", "/v2/deploymentPolicies", body, &created); err != nil {
		resp.Diagnostics.AddError("Create deployment policy failed", err.Error())
		return
	}
	// POST only returns deploymentPolicyId — read full state back.
	var got deploymentPolicyResp
	if err := r.c.Do(ctx, "GET", "/v2/deploymentPolicies/"+client.EscapeID(created.DeploymentPolicyID), nil, &got); err != nil {
		resp.Diagnostics.AddError("Read deployment policy after create failed", err.Error())
		return
	}
	diags := deploymentPolicyToState(ctx, &got, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got deploymentPolicyResp
	if err := r.c.Do(ctx, "GET", "/v2/deploymentPolicies/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read deployment policy failed", err.Error())
		return
	}
	diags := deploymentPolicyToState(ctx, &got, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *deploymentPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state deploymentPolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := patchDeploymentPolicyReq{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		v := plan.Name.ValueString()
		patch.Name = &v
	}
	if plan.NameInTenant.ValueString() != state.NameInTenant.ValueString() {
		v := plan.NameInTenant.ValueString()
		patch.NameInTenant = &v
	}
	if plan.CopyInputTableData.ValueBool() != state.CopyInputTableData.ValueBool() {
		v := plan.CopyInputTableData.ValueBool()
		patch.CopyInputTableData = &v
	}
	if plan.UseDependenciesWorkspace.ValueBool() != state.UseDependenciesWorkspace.ValueBool() {
		v := plan.UseDependenciesWorkspace.ValueBool()
		patch.UseDependenciesWorkspace = &v
	}
	if !plan.SourceSwapPolicies.Equal(state.SourceSwapPolicies) {
		var ids []string
		resp.Diagnostics.Append(plan.SourceSwapPolicies.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		patch.SourceSwapPolicies = &ids
	}

	var got deploymentPolicyResp
	if err := r.c.Do(ctx, "PATCH", "/v2/deploymentPolicies/"+client.EscapeID(state.ID.ValueString()), patch, &got); err != nil {
		resp.Diagnostics.AddError("Update deployment policy failed", err.Error())
		return
	}
	diags := deploymentPolicyToState(ctx, &got, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentPolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/deploymentPolicies/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete deployment policy failed", err.Error())
	}
}

func (r *deploymentPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func deploymentPolicyToState(ctx context.Context, s *deploymentPolicyResp, dst *deploymentPolicyModel) diag.Diagnostics {
	dst.ID = types.StringValue(s.DeploymentPolicyID)
	dst.Name = types.StringValue(s.Name)
	dst.NameInTenant = types.StringValue(s.NameInTenant)
	if s.VersionTagID != nil {
		dst.VersionTagID = types.StringValue(*s.VersionTagID)
	} else {
		dst.VersionTagID = types.StringNull()
	}
	dst.CopyInputTableData = types.BoolValue(s.CopyInputTableData)
	dst.UseDependenciesWorkspace = types.BoolValue(s.UseDependenciesWorkspace)
	swapPolicies, diags := types.ListValueFrom(ctx, types.StringType, s.SourceSwapPolicies)
	dst.SourceSwapPolicies = swapPolicies
	return diags
}
