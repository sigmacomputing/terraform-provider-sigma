// Package whoami implements data.sigma_whoami.
package whoami

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*whoamiDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*whoamiDataSource)(nil)
)

func NewDataSource() datasource.DataSource { return &whoamiDataSource{} }

type whoamiDataSource struct{ c *client.Client }

type whoamiModel struct {
	ID             types.String `tfsdk:"id"`
	UserID         types.String `tfsdk:"user_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
}

type whoamiResp struct {
	UserID         string `json:"userId"`
	OrganizationID string `json:"organizationId"`
}

func (d *whoamiDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_whoami"
}

func (d *whoamiDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Identity of the API client the provider is authenticated as. Useful to assert a configuration targets the intended organization.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, Description: "Same as `user_id`."},
			"user_id":         schema.StringAttribute{Computed: true, Description: "Member ID of the authenticated principal."},
			"organization_id": schema.StringAttribute{Computed: true, Description: "Organization ID the credentials belong to."},
		},
	}
}

func (d *whoamiDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *whoamiDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var got whoamiResp
	if err := d.c.Do(ctx, "GET", "/v2/whoami", nil, &got); err != nil {
		resp.Diagnostics.AddError("Read whoami failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &whoamiModel{
		ID:             types.StringValue(got.UserID),
		UserID:         types.StringValue(got.UserID),
		OrganizationID: types.StringValue(got.OrganizationID),
	})...)
}
