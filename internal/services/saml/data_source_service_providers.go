package saml

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*serviceProvidersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*serviceProvidersDataSource)(nil)
)

func NewServiceProvidersDataSource() datasource.DataSource { return &serviceProvidersDataSource{} }

type serviceProvidersDataSource struct{ c *client.Client }

type serviceProvidersModel struct {
	IDs types.List `tfsdk:"ids"`
}

type serviceProviderEntry struct {
	ID string `json:"samlServiceProviderId"`
}

func (d *serviceProvidersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_saml_service_providers"
}

func (d *serviceProvidersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists SAML service provider IDs for the organization (Beta API). Service providers are read-only; manage their certificates with sigma_saml_service_provider_certificate.",
		Attributes: map[string]schema.Attribute{
			"ids": schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "SAML service provider IDs."},
		},
	}
}

func (d *serviceProvidersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *serviceProvidersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPageToken[serviceProviderEntry](ctx, d.c, "/v2/saml/service-providers", client.PageSize(500))
	if err != nil {
		resp.Diagnostics.AddError("List SAML service providers failed", err.Error())
		return
	}
	ids := make([]string, 0, len(all))
	for _, e := range all {
		ids = append(ids, e.ID)
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, ids)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &serviceProvidersModel{IDs: list})...)
}
