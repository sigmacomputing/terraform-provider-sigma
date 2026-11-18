package connections

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*tableColsDS)(nil)
	_ datasource.DataSourceWithConfigure = (*tableColsDS)(nil)
)

func NewTableColumnsDataSource() datasource.DataSource { return &tableColsDS{} }

type tableColsDS struct{ c *client.Client }

type tableColsModel struct {
	TableID types.String    `tfsdk:"table_id"`
	Columns []tableColEntry `tfsdk:"columns"`
}
type tableColEntry struct {
	Name       types.String `tfsdk:"name"`
	TypeJSON   types.String `tfsdk:"type_json"`
	Visibility types.String `tfsdk:"visibility"`
}

type rawCol struct {
	Name       string          `json:"name"`
	Type       json.RawMessage `json:"type"`
	Visibility string          `json:"visibility"`
}

func (d *tableColsDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection_table_columns"
}
func (d *tableColsDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List columns for a connection table. The `type` field is a recursive object; surfaced as `type_json` for downstream JSON parsing.",
		Attributes: map[string]schema.Attribute{
			"table_id": schema.StringAttribute{Required: true, Description: "Connection table UUID (from `sigma_connection_path` or similar)."},
			"columns": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"name":       schema.StringAttribute{Computed: true},
					"type_json":  schema.StringAttribute{Computed: true, Description: "JSON-encoded column type spec (recursive)."},
					"visibility": schema.StringAttribute{Computed: true, Description: "`included`, `hidden`, or `removed`."},
				}},
			},
		},
	}
}
func (d *tableColsDS) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *tableColsDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data tableColsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Endpoint is non-paginated (entries list, no nextPage in our prior survey).
	// Use ListAllPaged anyway — it tolerates absent nextPage by returning after first page.
	all, err := client.ListAllPageToken[rawCol](ctx, d.c, "/v2/connections/tables/"+client.EscapeID(data.TableID.ValueString())+"/columns", client.PageSize(client.MaxPageSize))
	if err != nil {
		resp.Diagnostics.AddError("List table columns failed", err.Error())
		return
	}
	data.Columns = make([]tableColEntry, 0, len(all))
	for _, c := range all {
		data.Columns = append(data.Columns, tableColEntry{
			Name:       types.StringValue(c.Name),
			TypeJSON:   types.StringValue(string(c.Type)),
			Visibility: types.StringValue(c.Visibility),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
