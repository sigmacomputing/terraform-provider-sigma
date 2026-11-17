// Package tenants implements sigma_tenant.
package tenants

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
	_ resource.Resource                = (*tenantResource)(nil)
	_ resource.ResourceWithConfigure   = (*tenantResource)(nil)
	_ resource.ResourceWithImportState = (*tenantResource)(nil)
)

func NewResource() resource.Resource { return &tenantResource{} }

type tenantResource struct{ c *client.Client }

type tenantModel struct {
	ID            types.String `tfsdk:"id"`
	ParentOrgID   types.String `tfsdk:"parent_org_id"`
	Name          types.String `tfsdk:"name"`
	Slug          types.String `tfsdk:"slug"`
	CloudProvider types.String `tfsdk:"cloud_provider"`
	CreatedAt     types.String `tfsdk:"created_at"`
	UpdatedAt     types.String `tfsdk:"updated_at"`
}

type tenantResp struct {
	TenantOrganizationID   string    `json:"tenantOrganizationId"`
	ParentOrganizationID   string    `json:"parentOrganizationId"`
	TenantOrganizationName string    `json:"tenantOrganizationName"`
	TenantOrganizationSlug string    `json:"tenantOrganizationSlug"`
	CloudProvider          string    `json:"cloudProvider,omitempty"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

type createTenantReq struct {
	TenantOrganizationName string  `json:"tenantOrganizationName"`
	TenantOrganizationSlug string  `json:"tenantOrganizationSlug"`
	CloudProvider          *string `json:"cloudProvider,omitempty"`
}

type patchTenantReq struct {
	TenantOrganizationName *string `json:"tenantOrganizationName,omitempty"`
	TenantOrganizationSlug *string `json:"tenantOrganizationSlug,omitempty"`
}

func (r *tenantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tenant"
}

func (r *tenantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma tenant organization for embedded analytics. Tenants are child organizations provisioned under a parent org. `cloud_provider` is immutable after creation.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"parent_org_id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":          schema.StringAttribute{Required: true, Description: "Display name of the tenant organization."},
			"slug":          schema.StringAttribute{Required: true, Description: "URL identifier for the tenant organization. Must be unique within the parent org."},
			"cloud_provider": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Cloud provider to host the tenant in. One of: `aws`, `aws-au`, `aws-ca`, `aws-eu`, `aws-uk`, `aws-us-a`, `azure-ca`, `azure-eu`, `azure-uk`, `azure-us`, `gcp`, `gcp-us`. Defaults to the parent org's cloud provider."},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *tenantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *tenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan tenantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createTenantReq{
		TenantOrganizationName: plan.Name.ValueString(),
		TenantOrganizationSlug: plan.Slug.ValueString(),
	}
	if !plan.CloudProvider.IsNull() && !plan.CloudProvider.IsUnknown() {
		v := plan.CloudProvider.ValueString()
		body.CloudProvider = &v
	}
	var got tenantResp
	if err := r.c.Do(ctx, "POST", "/v2/tenants", body, &got); err != nil {
		resp.Diagnostics.AddError("Create tenant failed", err.Error())
		return
	}
	tenantToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state tenantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got tenantResp
	if err := r.c.Do(ctx, "GET", "/v2/tenants/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read tenant failed", err.Error())
		return
	}
	tenantToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *tenantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state tenantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := patchTenantReq{}
	if plan.Name.ValueString() != state.Name.ValueString() {
		v := plan.Name.ValueString()
		patch.TenantOrganizationName = &v
	}
	if plan.Slug.ValueString() != state.Slug.ValueString() {
		v := plan.Slug.ValueString()
		patch.TenantOrganizationSlug = &v
	}
	var got tenantResp
	if err := r.c.Do(ctx, "PATCH", "/v2/tenants/"+client.EscapeID(state.ID.ValueString()), patch, &got); err != nil {
		resp.Diagnostics.AddError("Update tenant failed", err.Error())
		return
	}
	tenantToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *tenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state tenantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/tenants/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete tenant failed", err.Error())
	}
}

func (r *tenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func tenantToState(t *tenantResp, dst *tenantModel) {
	dst.ID = types.StringValue(t.TenantOrganizationID)
	dst.ParentOrgID = types.StringValue(t.ParentOrganizationID)
	dst.Name = types.StringValue(t.TenantOrganizationName)
	dst.Slug = types.StringValue(t.TenantOrganizationSlug)
	dst.CloudProvider = types.StringValue(t.CloudProvider)
	dst.CreatedAt = types.StringValue(t.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(t.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
}
