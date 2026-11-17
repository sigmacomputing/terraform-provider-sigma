package deploymentpolicies

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
	_ resource.Resource                = (*deploymentPolicyTenantResource)(nil)
	_ resource.ResourceWithConfigure   = (*deploymentPolicyTenantResource)(nil)
	_ resource.ResourceWithImportState = (*deploymentPolicyTenantResource)(nil)
)

func NewTenantResource() resource.Resource { return &deploymentPolicyTenantResource{} }

type deploymentPolicyTenantResource struct{ c *client.Client }

type deploymentPolicyTenantModel struct {
	ID                   types.String `tfsdk:"id"`
	DeploymentPolicyID   types.String `tfsdk:"deployment_policy_id"`
	TenantOrganizationID types.String `tfsdk:"tenant_organization_id"`
}

func (r *deploymentPolicyTenantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_policy_tenant"
}

func (r *deploymentPolicyTenantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Targets a tenant organization for a deployment policy. Composite ID `<deployment_policy_id>:<tenant_organization_id>`. Look up available tenant orgs with data.sigma_deployment_policy_tenants.",
		Attributes: map[string]schema.Attribute{
			"id":                     schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"deployment_policy_id":   schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"tenant_organization_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "ID of the tenant organization to target."},
		},
	}
}

func (r *deploymentPolicyTenantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *deploymentPolicyTenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentPolicyTenantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policyID := plan.DeploymentPolicyID.ValueString()
	tenantID := plan.TenantOrganizationID.ValueString()
	listPath := "/v2/deploymentPolicies/" + client.EscapeID(policyID) + "/tenants"

	defer client.InvalidateList(r.c, deploymentPolicyTenantsKey(policyID))
	if err := r.c.Do(ctx, "POST", listPath, addTenantReq{TenantOrganizationID: tenantID}, nil); err != nil {
		resp.Diagnostics.AddError("Assign tenant to deployment policy failed", err.Error())
		return
	}
	// tenantOrganizationId is client-supplied, not server-minted — just confirm it landed.
	err := client.Eventually(ctx, func() (bool, error) {
		all, err := client.ListAllPageToken[string](ctx, r.c, listPath, client.PageSize(500))
		if err != nil {
			return false, err
		}
		for _, id := range all {
			if id == tenantID {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not confirm tenant assignment",
			fmt.Sprintf("Assigned tenant %s to deployment policy %s but it does not appear in the post-assign tenant list after retrying: %s", tenantID, policyID, err.Error()),
		)
		return
	}
	plan.ID = types.StringValue(policyID + ":" + tenantID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentPolicyTenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentPolicyTenantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policyID := state.DeploymentPolicyID.ValueString()
	tenantID := state.TenantOrganizationID.ValueString()
	all, err := client.CachedList(r.c, deploymentPolicyTenantsKey(policyID), func() ([]string, error) {
		return client.ListAllPageToken[string](ctx, r.c, "/v2/deploymentPolicies/"+client.EscapeID(policyID)+"/tenants", client.PageSize(500))
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List deployment policy tenants failed", err.Error())
		return
	}
	for _, id := range all {
		if id == tenantID {
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *deploymentPolicyTenantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// RequiresReplace on everything; no PATCH exists, so this is a safe no-op.
}

func (r *deploymentPolicyTenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentPolicyTenantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policyID := state.DeploymentPolicyID.ValueString()
	defer client.InvalidateList(r.c, deploymentPolicyTenantsKey(policyID))
	p := "/v2/deploymentPolicies/" + client.EscapeID(policyID) + "/tenants/" + client.EscapeID(state.TenantOrganizationID.ValueString())
	if err := r.c.Do(ctx, "DELETE", p, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Remove tenant from deployment policy failed", err.Error())
	}
}

func (r *deploymentPolicyTenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<deployment_policy_id>:<tenant_organization_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deployment_policy_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_organization_id"), parts[1])...)
}

// deploymentPolicyTenantsKey is the CachedList key for a policy's tenant associations.
func deploymentPolicyTenantsKey(policyID string) string { return "deploy-policy-tenants:" + policyID }
