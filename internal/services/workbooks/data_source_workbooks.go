package workbooks

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*workbooksDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workbooksDataSource)(nil)
)

func NewWorkbooksDataSource() datasource.DataSource { return &workbooksDataSource{} }

type workbooksDataSource struct{ c *client.Client }

type workbooksDSModel struct {
	Workbooks []workbookDSEntry `tfsdk:"workbooks"`
}
type workbookDSEntry struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
	Path types.String `tfsdk:"path"`
}

func (d *workbooksDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workbooks"
}
func (d *workbooksDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all workbooks visible to the API caller.",
		Attributes: map[string]schema.Attribute{
			"workbooks": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true},
					"name": schema.StringAttribute{Computed: true},
					"path": schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}
func (d *workbooksDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *workbooksDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPaged[workbook](ctx, d.c, "/v2/workbooks", nil)
	if err != nil {
		resp.Diagnostics.AddError("List workbooks failed", err.Error())
		return
	}
	out := workbooksDSModel{Workbooks: make([]workbookDSEntry, 0, len(all))}
	for _, w := range all {
		out.Workbooks = append(out.Workbooks, workbookDSEntry{
			ID: types.StringValue(w.WorkbookID), Name: types.StringValue(w.Name), Path: types.StringValue(w.Path),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
