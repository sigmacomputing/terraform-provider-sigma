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
	_ datasource.DataSource              = (*membersDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*membersDataSource)(nil)
)

func NewMembersDataSource() datasource.DataSource { return &membersDataSource{} }

type membersDataSource struct{ c *client.Client }

type membersDSModel struct {
	Members []memberDSEntry `tfsdk:"members"`
}

type memberDSEntry struct {
	ID         types.String `tfsdk:"id"`
	Email      types.String `tfsdk:"email"`
	FirstName  types.String `tfsdk:"first_name"`
	LastName   types.String `tfsdk:"last_name"`
	MemberType types.String `tfsdk:"member_type"`
	UserKind   types.String `tfsdk:"user_kind"`
}

func (d *membersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_members"
}

func (d *membersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all members in the Sigma organization.",
		Attributes: map[string]schema.Attribute{
			"members": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true},
						"email":       schema.StringAttribute{Computed: true},
						"first_name":  schema.StringAttribute{Computed: true},
						"last_name":   schema.StringAttribute{Computed: true},
						"member_type": schema.StringAttribute{Computed: true},
						"user_kind":   schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *membersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *membersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPaged[member](ctx, d.c, "/v2.1/members", nil)
	if err != nil {
		resp.Diagnostics.AddError("List members failed", err.Error())
		return
	}
	out := membersDSModel{Members: make([]memberDSEntry, 0, len(all))}
	for _, m := range all {
		out.Members = append(out.Members, memberDSEntry{
			ID:         types.StringValue(m.MemberID),
			Email:      types.StringValue(m.Email),
			FirstName:  types.StringValue(m.FirstName),
			LastName:   types.StringValue(m.LastName),
			MemberType: types.StringValue(m.MemberType),
			UserKind:   types.StringValue(m.UserKind),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
