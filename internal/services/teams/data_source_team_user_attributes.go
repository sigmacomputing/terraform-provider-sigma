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
	_ datasource.DataSource              = (*teamUADS)(nil)
	_ datasource.DataSourceWithConfigure = (*teamUADS)(nil)
)

func NewTeamUserAttributesDataSource() datasource.DataSource { return &teamUADS{} }

type teamUADS struct{ c *client.Client }

type teamUADSModel struct {
	TeamID         types.String  `tfsdk:"team_id"`
	UserAttributes []teamUAEntry `tfsdk:"user_attributes"`
}
type teamUAEntry struct {
	ID    types.String `tfsdk:"id"`
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

type teamUAResponse struct {
	UserAttributeID   string `json:"userAttributeId"`
	UserAttributeName string `json:"userAttributeName"`
	Value             struct {
		Val  string `json:"val"`
		Type string `json:"type"`
	} `json:"value"`
}

func (d *teamUADS) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_user_attributes"
}
func (d *teamUADS) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List the user attributes assigned to a team, with their assigned values.",
		Attributes: map[string]schema.Attribute{
			"team_id": schema.StringAttribute{Required: true, Description: "Team UUID."},
			"user_attributes": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":    schema.StringAttribute{Computed: true},
					"name":  schema.StringAttribute{Computed: true},
					"value": schema.StringAttribute{Computed: true, Sensitive: true},
				}},
			},
		},
	}
}
func (d *teamUADS) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *teamUADS) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data teamUADSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := client.ListAllPaged[teamUAResponse](ctx, d.c, "/v2/teams/"+client.EscapeID(data.TeamID.ValueString())+"/user-attributes", nil)
	if err != nil {
		resp.Diagnostics.AddError("List team user attributes failed", err.Error())
		return
	}
	data.UserAttributes = make([]teamUAEntry, 0, len(all))
	for _, e := range all {
		data.UserAttributes = append(data.UserAttributes, teamUAEntry{
			ID: types.StringValue(e.UserAttributeID), Name: types.StringValue(e.UserAttributeName), Value: types.StringValue(e.Value.Val),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
