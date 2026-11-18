package workspaces

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*workspacesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workspacesDataSource)(nil)
)

func NewWorkspacesDataSource() datasource.DataSource { return &workspacesDataSource{} }

type workspacesDataSource struct{ c *client.Client }

type workspacesDSModel struct {
	Workspaces []workspaceDSEntry `tfsdk:"workspaces"`
}

type workspaceDSEntry struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (d *workspacesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspaces"
}

func (d *workspacesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all workspaces in the organization.",
		Attributes: map[string]schema.Attribute{
			"workspaces": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":   schema.StringAttribute{Computed: true},
						"name": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *workspacesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *workspacesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPaged[workspace](ctx, d.c, "/v2.1/workspaces", nil)
	if err != nil {
		resp.Diagnostics.AddError("List workspaces failed", err.Error())
		return
	}
	out := workspacesDSModel{Workspaces: make([]workspaceDSEntry, 0, len(all))}
	for _, w := range all {
		out.Workspaces = append(out.Workspaces, workspaceDSEntry{
			ID:   types.StringValue(w.WorkspaceID),
			Name: types.StringValue(w.Name),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
