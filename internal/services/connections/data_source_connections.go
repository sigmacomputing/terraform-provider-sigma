package connections

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*connectionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectionsDataSource)(nil)
)

func NewConnectionsDataSource() datasource.DataSource { return &connectionsDataSource{} }

type connectionsDataSource struct{ c *client.Client }

type connectionsDSModel struct {
	Connections []connectionDSEntry `tfsdk:"connections"`
}

type connectionDSEntry struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	IsSample   types.Bool   `tfsdk:"is_sample"`
	IsAuditLog types.Bool   `tfsdk:"is_audit_log"`
	UseOauth   types.Bool   `tfsdk:"use_oauth"`
}

func (d *connectionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connections"
}

func (d *connectionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all connections in the organization. Use it to resolve a connection's `id` by `name` " +
			"(e.g. `one([for c in data.sigma_connections.all.connections : c.id if c.name == \"Snowflake\"])`) to feed " +
			"`sigma_connection_grant` or `sigma_connection_path`.",
		Attributes: map[string]schema.Attribute{
			"connections": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":           schema.StringAttribute{Computed: true, Description: "Connection UUID."},
						"name":         schema.StringAttribute{Computed: true},
						"type":         schema.StringAttribute{Computed: true, Description: "Warehouse type, e.g. `snowflake`, `bigquery`, `postgres`."},
						"is_sample":    schema.BoolAttribute{Computed: true},
						"is_audit_log": schema.BoolAttribute{Computed: true},
						"use_oauth":    schema.BoolAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *connectionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPaged[connection](ctx, d.c, "/v2/connections", nil)
	if err != nil {
		resp.Diagnostics.AddError("List connections failed", err.Error())
		return
	}
	out := connectionsDSModel{Connections: make([]connectionDSEntry, 0, len(all))}
	for _, c := range all {
		out.Connections = append(out.Connections, connectionDSEntry{
			ID:         types.StringValue(c.ConnectionID),
			Name:       types.StringValue(c.Name),
			Type:       types.StringValue(c.Type),
			IsSample:   types.BoolValue(c.IsSample),
			IsAuditLog: types.BoolValue(c.IsAuditLog),
			UseOauth:   types.BoolValue(c.UseOauth),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
