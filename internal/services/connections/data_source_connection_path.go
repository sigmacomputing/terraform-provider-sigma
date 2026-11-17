package connections

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*connectionPathDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectionPathDataSource)(nil)
)

func NewPathDataSource() datasource.DataSource { return &connectionPathDataSource{} }

type connectionPathDataSource struct{ c *client.Client }

type connectionPathDSModel struct {
	ConnectionID     types.String `tfsdk:"connection_id"`
	Path             types.String `tfsdk:"path"`
	ConnectionPathID types.String `tfsdk:"connection_path_id"`
}

// Sigma's GET /v2/connections/paths returns each path with `urlId` as the
// identifier (also surfaced as `inodeId` on grant list entries), and `path`
// as an ARRAY of strings representing the breadcrumb (e.g. ["DB","SCHEMA"]).
// We accept the user-supplied path as a dot-joined string and join the array
// for matching.
type connectionPath struct {
	URLID        string   `json:"urlId"`
	ConnectionID string   `json:"connectionId"`
	Path         []string `json:"path"`
}

func (d *connectionPathDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection_path"
}

func (d *connectionPathDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a connection path UUID by its (connection_id, path) tuple. Use the result for `sigma_connection_path_grant.connection_path_id`.",
		Attributes: map[string]schema.Attribute{
			"connection_id": schema.StringAttribute{Required: true},
			"path": schema.StringAttribute{Required: true,
				Description: "The path within the connection, e.g. `database.schema` or `database.schema.table`."},
			"connection_path_id": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *connectionPathDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectionPathDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data connectionPathDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// NB: Sigma's `connectionId` query param appears to be ignored — the
	// endpoint returns every connection's paths regardless. We filter
	// client-side and STOP early on first match — without this, looking up a
	// single path on a busy org takes tens of requests (~50 entries/page) and
	// is a multi-second tax on every `terraform plan`.
	q := url.Values{}
	q.Set("connectionId", data.ConnectionID.ValueString())
	wanted := data.Path.ValueString()
	wantedConn := data.ConnectionID.ValueString()

	var foundID string
	err := client.ForEachPaged[connectionPath](ctx, d.c, "/v2/connections/paths", q, func(p connectionPath) (bool, error) {
		if p.ConnectionID == wantedConn && strings.Join(p.Path, ".") == wanted {
			foundID = p.URLID
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("List connection paths failed", err.Error())
		return
	}
	if foundID != "" {
		data.ConnectionPathID = types.StringValue(foundID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}
	resp.Diagnostics.AddError(
		"Connection path not found",
		fmt.Sprintf("No path %q found under connection %q. Verify the dot-joined path string matches a synced path "+
			"(e.g. `DATABASE.SCHEMA.TABLE`) and that the connection has finished its initial sync.",
			wanted, wantedConn),
	)
}
