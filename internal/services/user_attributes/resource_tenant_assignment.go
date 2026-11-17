package user_attributes

import (
	"context"
	"fmt"
	"net/url"
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
	_ resource.Resource                = (*tenantAssignmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*tenantAssignmentResource)(nil)
	_ resource.ResourceWithImportState = (*tenantAssignmentResource)(nil)
)

func NewTenantAssignmentResource() resource.Resource { return &tenantAssignmentResource{} }

type tenantAssignmentResource struct{ c *client.Client }

type tenantAssignmentModel struct {
	ID                   types.String `tfsdk:"id"`
	UserAttributeID      types.String `tfsdk:"user_attribute_id"`
	TenantOrganizationID types.String `tfsdk:"tenant_organization_id"`
	Value                types.String `tfsdk:"value"`
}

// tenantAssignment is one entry in a /tenants assignments POST or GET.
type tenantAssignment struct {
	TenantOrganizationID string       `json:"tenantOrganizationId"`
	Value                defaultValue `json:"value"`
}

func (r *tenantAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute_tenant_assignment"
}

func (r *tenantAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a user attribute value to a tenant organization. Composite ID `<user_attribute_id>:<tenant_organization_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"user_attribute_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"tenant_organization_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"value": schema.StringAttribute{Required: true, Sensitive: true,
				Description: "String value assigned to this tenant. Marked Sensitive like the other assignment values."},
		},
	}
}

func (r *tenantAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// upsert POSTs the assignment (Sigma's POST overwrites an existing value).
func (r *tenantAssignmentResource) upsert(ctx context.Context, plan *tenantAssignmentModel) error {
	uaID := plan.UserAttributeID.ValueString()
	body := assignmentsBody[tenantAssignment]{Assignments: []tenantAssignment{{
		TenantOrganizationID: plan.TenantOrganizationID.ValueString(),
		Value:                defaultValue{Val: plan.Value.ValueString(), Type: "string"},
	}}}
	defer client.InvalidateList(r.c, uaTenantsKey(uaID))
	return r.c.Do(ctx, "POST", "/v2/user-attributes/"+client.EscapeID(uaID)+"/tenants", body, nil)
}

func (r *tenantAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tenantAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Assign user attribute to tenant failed", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.UserAttributeID.ValueString() + ":" + plan.TenantOrganizationID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tenantAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tenantAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := state.UserAttributeID.ValueString()
	all, err := client.CachedList(r.c, uaTenantsKey(uaID), func() ([]tenantAssignment, error) {
		return client.ListAllPaged[tenantAssignment](ctx, r.c, "/v2/user-attributes/"+client.EscapeID(uaID)+"/tenants", url.Values{})
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List tenant assignments failed", err.Error())
		return
	}
	for _, a := range all {
		if a.TenantOrganizationID == state.TenantOrganizationID.ValueString() {
			state.Value = types.StringValue(a.Value.Val)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *tenantAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan tenantAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.upsert(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Update tenant assignment failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tenantAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tenantAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := state.UserAttributeID.ValueString()
	defer client.InvalidateList(r.c, uaTenantsKey(uaID))
	p := "/v2/user-attributes/" + client.EscapeID(uaID) + "/tenants/" + client.EscapeID(state.TenantOrganizationID.ValueString())
	if err := r.c.Do(ctx, "DELETE", p, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete tenant assignment failed", err.Error())
	}
}

// uaTenantsKey is the per-run CachedList key for a user attribute's tenant assignments.
func uaTenantsKey(uaID string) string { return "ua-tenants:" + uaID }

func (r *tenantAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<user_attribute_id>:<tenant_organization_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_attribute_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tenant_organization_id"), parts[1])...)
}
