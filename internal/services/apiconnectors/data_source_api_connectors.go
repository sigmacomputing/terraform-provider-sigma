package apiconnectors

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*connectorsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectorsDataSource)(nil)
)

func NewConnectorsDataSource() datasource.DataSource { return &connectorsDataSource{} }

type connectorsDataSource struct{ c *client.Client }

type connectorsDSModel struct {
	Connectors []connectorDSEntry `tfsdk:"connectors"`
}
type connectorDSEntry struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	AuthID      types.String `tfsdk:"auth_id"`
}

func (d *connectorsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_connectors"
}
func (d *connectorsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all API connectors.",
		Attributes: map[string]schema.Attribute{
			"connectors": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          schema.StringAttribute{Computed: true},
					"name":        schema.StringAttribute{Computed: true},
					"description": schema.StringAttribute{Computed: true},
					"auth_id":     schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}
func (d *connectorsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *connectorsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPageToken[apiConnector](ctx, d.c, "/v2/api-connectors", client.PageSize(client.MaxPageSize))
	if err != nil {
		resp.Diagnostics.AddError("List API connectors failed", err.Error())
		return
	}
	out := connectorsDSModel{Connectors: make([]connectorDSEntry, 0, len(all))}
	for _, c := range all {
		e := connectorDSEntry{
			ID:          types.StringValue(c.APIConnectorID),
			Name:        types.StringValue(c.Name),
			Description: types.StringValue(c.Description),
			AuthID:      types.StringNull(),
		}
		if c.AuthID != nil {
			e.AuthID = types.StringValue(*c.AuthID)
		}
		out.Connectors = append(out.Connectors, e)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
