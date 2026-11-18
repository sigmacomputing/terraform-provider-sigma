package tenants

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*deploymentCapabilityResource)(nil)
	_ resource.ResourceWithConfigure   = (*deploymentCapabilityResource)(nil)
	_ resource.ResourceWithImportState = (*deploymentCapabilityResource)(nil)
)

func NewDeploymentCapabilityResource() resource.Resource { return &deploymentCapabilityResource{} }

type deploymentCapabilityResource struct{ c *client.Client }

type deploymentCapabilityModel struct {
	ID                   types.String `tfsdk:"id"`
	TenantOrganizationID types.String `tfsdk:"tenant_organization_id"`
	DeployToTenantID     types.String `tfsdk:"deploy_to_tenant_organization_id"`
}

type capabilitiesReq struct {
	DeployToTenantOrganizationIDs []string `json:"deployToTenantOrganizationIds"`
}

type capabilityEntry struct {
	TenantOrganizationID string `json:"tenantOrganizationId"`
}

func capabilitiesPath(tenantID string) string {
	return "/v2/tenants/" + client.EscapeID(tenantID) + "/capabilities/deployments"
}

func capabilitiesKey(tenantID string) string { return "tenant-deploy-capabilities:" + tenantID }

func (r *deploymentCapabilityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant_deployment_capability"
}

func (r *deploymentCapabilityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Allows a tenant organization to deploy content to another tenant organization. Composite ID `<tenant_organization_id>:<deploy_to_tenant_organization_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"tenant_organization_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Tenant organization that is granted the capability."},
			"deploy_to_tenant_organization_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Tenant organization it may deploy to."},
		},
	}
}

func (r *deploymentCapabilityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *deploymentCapabilityResource) listTargets(ctx context.Context, tenantID string) ([]capabilityEntry, error) {
	return client.CachedList(r.c, capabilitiesKey(tenantID), func() ([]capabilityEntry, error) {
		return client.ListAllPageToken[capabilityEntry](ctx, r.c, capabilitiesPath(tenantID), client.PageSize(client.MaxPageSize))
	})
}

func (r *deploymentCapabilityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentCapabilityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tenantID := plan.TenantOrganizationID.ValueString()
	target := plan.DeployToTenantID.ValueString()
	defer client.InvalidateList(r.c, capabilitiesKey(tenantID))
	body := capabilitiesReq{DeployToTenantOrganizationIDs: []string{target}}
	if err := r.c.Do(ctx, "POST", capabilitiesPath(tenantID)+":batchAdd", body, nil); err != nil {
		resp.Diagnostics.AddError("Add tenant deployment capability failed", err.Error())
		return
	}
	plan.ID = types.StringValue(tenantID + ":" + target)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentCapabilityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentCapabilityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := r.listTargets(ctx, state.TenantOrganizationID.ValueString())
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List tenant deployment capabilities failed", err.Error())
		return
	}
	for _, e := range all {
		if e.TenantOrganizationID == state.DeployToTenantID.ValueString() {
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *deploymentCapabilityResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// RequiresReplace on everything; Update is unreachable.
}

func (r *deploymentCapabilityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentCapabilityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	tenantID := state.TenantOrganizationID.ValueString()
	defer client.InvalidateList(r.c, capabilitiesKey(tenantID))
	body := capabilitiesReq{DeployToTenantOrganizationIDs: []string{state.DeployToTenantID.ValueString()}}
	if err := r.c.Do(ctx, "POST", capabilitiesPath(tenantID)+":batchRemove", body, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Remove tenant deployment capability failed", err.Error())
	}
}

func (r *deploymentCapabilityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<tenant_organization_id>:<deploy_to_tenant_organization_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_organization_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deploy_to_tenant_organization_id"), parts[1])...)
}
