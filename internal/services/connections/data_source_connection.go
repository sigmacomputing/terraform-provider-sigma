package connections

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*connectionDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*connectionDataSource)(nil)
)

func NewConnectionDataSource() datasource.DataSource { return &connectionDataSource{} }

type connectionDataSource struct{ c *client.Client }

type connectionDSModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	IsSample   types.Bool   `tfsdk:"is_sample"`
	IsAuditLog types.Bool   `tfsdk:"is_audit_log"`
	UseOauth   types.Bool   `tfsdk:"use_oauth"`
}

func (d *connectionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection"
}

func (d *connectionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a single connection by `id` (preferred when known) or by `name`. Set exactly one. " +
			"`name` lookups page through `/v2/connections` client-side and error if the name is not unique " +
			"(connection names are not guaranteed unique — use `id` to disambiguate).",
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Optional: true, Computed: true, Description: "Connection UUID."},
			"name":         schema.StringAttribute{Optional: true, Computed: true, Description: "Connection display name. Matched exactly."},
			"type":         schema.StringAttribute{Computed: true, Description: "Warehouse type, e.g. `snowflake`, `bigquery`, `postgres`."},
			"is_sample":    schema.BoolAttribute{Computed: true},
			"is_audit_log": schema.BoolAttribute{Computed: true},
			"use_oauth":    schema.BoolAttribute{Computed: true},
		},
	}
}

func (d *connectionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *connectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data connectionDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasID := !data.ID.IsNull() && data.ID.ValueString() != ""
	hasName := !data.Name.IsNull() && data.Name.ValueString() != ""
	if hasID == hasName {
		resp.Diagnostics.AddError("Invalid lookup", "Set exactly one of `id` or `name`.")
		return
	}

	// Fast path: direct GET by id.
	if hasID {
		var got connection
		if err := d.c.Do(ctx, "GET", "/v2/connections/"+client.EscapeID(data.ID.ValueString()), nil, &got); err != nil {
			resp.Diagnostics.AddError("Get connection failed", err.Error())
			return
		}
		applyConnection(&got, &data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	// Lookup by name: list and require exactly one match. Names aren't guaranteed
	// unique, so we collect all matches rather than short-circuiting.
	wanted := data.Name.ValueString()
	// `search` narrows server-side (substring); the exact match is checked below.
	all, err := client.ListAllPaged[connection](ctx, d.c, "/v2/connections", url.Values{"search": {wanted}})
	if err != nil {
		resp.Diagnostics.AddError("List connections failed", err.Error())
		return
	}
	var matches []connection
	for _, c := range all {
		if c.Name == wanted {
			matches = append(matches, c)
		}
	}
	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError("Connection not found", fmt.Sprintf("No connection named %q.", wanted))
	case 1:
		applyConnection(&matches[0], &data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	default:
		resp.Diagnostics.AddError("Connection name not unique",
			fmt.Sprintf("Found %d connections named %q; look it up by `id` instead.", len(matches), wanted))
	}
}

func applyConnection(c *connection, dst *connectionDSModel) {
	dst.ID = types.StringValue(c.ConnectionID)
	dst.Name = types.StringValue(c.Name)
	dst.Type = types.StringValue(c.Type)
	dst.IsSample = types.BoolValue(c.IsSample)
	dst.IsAuditLog = types.BoolValue(c.IsAuditLog)
	dst.UseOauth = types.BoolValue(c.UseOauth)
}
