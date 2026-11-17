package deploymentpolicies

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*deploymentPolicyTenantsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*deploymentPolicyTenantsDataSource)(nil)
)

func NewTenantsDataSource() datasource.DataSource { return &deploymentPolicyTenantsDataSource{} }

type deploymentPolicyTenantsDataSource struct{ c *client.Client }

type deploymentPolicyTenantsDSModel struct {
	Tenants []tenantOrgDSEntry `tfsdk:"tenants"`
}

type tenantOrgDSEntry struct {
	TenantOrganizationID   types.String `tfsdk:"tenant_organization_id"`
	TenantOrganizationSlug types.String `tfsdk:"tenant_organization_slug"`
	TenantOrganizationName types.String `tfsdk:"tenant_organization_name"`
}

func (d *deploymentPolicyTenantsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_policy_tenants"
}

func (d *deploymentPolicyTenantsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists tenant organizations available to target with a sigma_deployment_policy_tenant, for looking up tenant_organization_id by slug or name.",
		Attributes: map[string]schema.Attribute{
			"tenants": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"tenant_organization_id":   schema.StringAttribute{Computed: true},
						"tenant_organization_slug": schema.StringAttribute{Computed: true},
						"tenant_organization_name": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *deploymentPolicyTenantsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.c = c
}

func (d *deploymentPolicyTenantsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPageToken[tenantOrgEntry](ctx, d.c, "/v2/deploymentPolicies/tenants", client.PageSize(500))
	if err != nil {
		resp.Diagnostics.AddError("List deployment policy tenants failed", err.Error())
		return
	}
	out := deploymentPolicyTenantsDSModel{Tenants: make([]tenantOrgDSEntry, 0, len(all))}
	for _, t := range all {
		out.Tenants = append(out.Tenants, tenantOrgDSEntry{
			TenantOrganizationID:   types.StringValue(t.TenantOrganizationID),
			TenantOrganizationSlug: types.StringValue(t.TenantOrganizationSlug),
			TenantOrganizationName: types.StringValue(t.TenantOrganizationName),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
