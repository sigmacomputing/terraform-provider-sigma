package teams

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*teamsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*teamsDataSource)(nil)
)

func NewTeamsDataSource() datasource.DataSource { return &teamsDataSource{} }

type teamsDataSource struct{ c *client.Client }

type teamsDSModel struct {
	Teams []teamDSEntry `tfsdk:"teams"`
}

type teamDSEntry struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Visibility types.String `tfsdk:"visibility"`
}

func (d *teamsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_teams"
}

func (d *teamsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all teams in the Sigma organization.",
		Attributes: map[string]schema.Attribute{
			"teams": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":         schema.StringAttribute{Computed: true},
						"name":       schema.StringAttribute{Computed: true},
						"visibility": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *teamsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *teamsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPaged[team](ctx, d.c, "/v2.1/teams", nil)
	if err != nil {
		resp.Diagnostics.AddError("List teams failed", err.Error())
		return
	}
	out := teamsDSModel{Teams: make([]teamDSEntry, 0, len(all))}
	for _, t := range all {
		out.Teams = append(out.Teams, teamDSEntry{
			ID:         types.StringValue(t.TeamID),
			Name:       types.StringValue(t.Name),
			Visibility: types.StringValue(t.Visibility),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
