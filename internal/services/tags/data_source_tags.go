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
	_ datasource.DataSource              = (*tagsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*tagsDataSource)(nil)
)

func NewTagsDataSource() datasource.DataSource { return &tagsDataSource{} }

type tagsDataSource struct{ c *client.Client }

type tagsDSModel struct {
	Tags []tagDSEntry `tfsdk:"tags"`
}
type tagDSEntry struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *tagsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tags"
}
func (d *tagsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all version tags.",
		Attributes: map[string]schema.Attribute{
			"tags": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":   schema.StringAttribute{Computed: true},
					"name": schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}
func (d *tagsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *tagsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPaged[tag](ctx, d.c, "/v2/tags", nil)
	if err != nil {
		resp.Diagnostics.AddError("List tags failed", err.Error())
		return
	}
	out := tagsDSModel{Tags: make([]tagDSEntry, 0, len(all))}
	for _, t := range all {
		out.Tags = append(out.Tags, tagDSEntry{ID: types.StringValue(t.VersionTagID), Name: types.StringValue(t.Name)})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
