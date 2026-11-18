package tags

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*tagWorkbooksDS)(nil)
	_ datasource.DataSourceWithConfigure = (*tagWorkbooksDS)(nil)
)

func NewTagWorkbooksDataSource() datasource.DataSource { return &tagWorkbooksDS{} }

type tagWorkbooksDS struct{ c *client.Client }

type tagWorkbooksModel struct {
	TagID     types.String       `tfsdk:"tag_id"`
	Workbooks []tagWorkbookEntry `tfsdk:"workbooks"`
}
type tagWorkbookEntry struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Path          types.String `tfsdk:"path"`
	LatestVersion types.Int64  `tfsdk:"latest_version"`
}

type taggedWorkbook struct {
	WorkbookID    string  `json:"workbookId"`
	Name          string  `json:"name"`
	Path          string  `json:"path"`
	LatestVersion float64 `json:"latestVersion"`
}

func (d *tagWorkbooksDS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag_workbooks"
}
func (d *tagWorkbooksDS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List workbooks tagged with a given version tag.",
		Attributes: map[string]schema.Attribute{
			"tag_id": schema.StringAttribute{Required: true, Description: "Version tag UUID."},
			"workbooks": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":             schema.StringAttribute{Computed: true},
					"name":           schema.StringAttribute{Computed: true},
					"path":           schema.StringAttribute{Computed: true},
					"latest_version": schema.Int64Attribute{Computed: true},
				}},
			},
		},
	}
}
func (d *tagWorkbooksDS) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *tagWorkbooksDS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data tagWorkbooksModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := client.ListAllPaged[taggedWorkbook](ctx, d.c, "/v2/tags/"+client.EscapeID(data.TagID.ValueString())+"/workbooks", nil)
	if err != nil {
		resp.Diagnostics.AddError("List tag workbooks failed", err.Error())
		return
	}
	data.Workbooks = make([]tagWorkbookEntry, 0, len(all))
	for _, w := range all {
		data.Workbooks = append(data.Workbooks, tagWorkbookEntry{
			ID: types.StringValue(w.WorkbookID), Name: types.StringValue(w.Name),
			Path: types.StringValue(w.Path), LatestVersion: types.Int64Value(int64(w.LatestVersion)),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
