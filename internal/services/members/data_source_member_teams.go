package members

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*memberTeamsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*memberTeamsDataSource)(nil)
)

func NewMemberTeamsDataSource() datasource.DataSource { return &memberTeamsDataSource{} }

type memberTeamsDataSource struct{ c *client.Client }

type memberTeamsDSModel struct {
	MemberID types.String       `tfsdk:"member_id"`
	Teams    []memberTeamsEntry `tfsdk:"teams"`
}
type memberTeamsEntry struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Visibility types.String `tfsdk:"visibility"`
}

type memberTeamEntry struct {
	TeamID     string `json:"teamId"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"`
}

func (d *memberTeamsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member_teams"
}

func (d *memberTeamsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List the teams that a member belongs to.",
		Attributes: map[string]schema.Attribute{
			"member_id": schema.StringAttribute{Required: true, Description: "Member UUID."},
			"teams": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":         schema.StringAttribute{Computed: true},
					"name":       schema.StringAttribute{Computed: true},
					"visibility": schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *memberTeamsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *memberTeamsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data memberTeamsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := client.ListAllPaged[memberTeamEntry](ctx, d.c, "/v2/members/"+client.EscapeID(data.MemberID.ValueString())+"/teams", nil)
	if err != nil {
		resp.Diagnostics.AddError("List member teams failed", err.Error())
		return
	}
	data.Teams = make([]memberTeamsEntry, 0, len(all))
	for _, t := range all {
		data.Teams = append(data.Teams, memberTeamsEntry{
			ID: types.StringValue(t.TeamID), Name: types.StringValue(t.Name), Visibility: types.StringValue(t.Visibility),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
