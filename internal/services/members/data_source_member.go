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
	_ datasource.DataSource              = (*memberDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*memberDataSource)(nil)
)

func NewMemberDataSource() datasource.DataSource { return &memberDataSource{} }

type memberDataSource struct{ c *client.Client }

type memberDSModel struct {
	ID         types.String `tfsdk:"id"`
	Email      types.String `tfsdk:"email"`
	FirstName  types.String `tfsdk:"first_name"`
	LastName   types.String `tfsdk:"last_name"`
	MemberType types.String `tfsdk:"member_type"`
	UserKind   types.String `tfsdk:"user_kind"`
}

func (d *memberDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member"
}

func (d *memberDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a single member by `id` (preferred when known) or by `email`. Email lookups are case-insensitive and page through `/v2.1/members` client-side — they short-circuit on the first match, but on large orgs `id` lookups are faster.",
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Optional: true, Computed: true, Description: "Sigma member UUID."},
			"email":       schema.StringAttribute{Optional: true, Computed: true, Description: "Member's email address. Matched case-insensitively."},
			"first_name":  schema.StringAttribute{Computed: true},
			"last_name":   schema.StringAttribute{Computed: true},
			"member_type": schema.StringAttribute{Computed: true},
			"user_kind":   schema.StringAttribute{Computed: true},
		},
	}
}

func (d *memberDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *memberDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data memberDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasID := !data.ID.IsNull() && data.ID.ValueString() != ""
	hasEmail := !data.Email.IsNull() && data.Email.ValueString() != ""
	if !hasID && !hasEmail {
		resp.Diagnostics.AddError("Missing input", "Either `id` or `email` must be set.")
		return
	}

	if hasID {
		var got member
		if err := d.c.Do(ctx, "GET", "/v2/members/"+client.EscapeID(data.ID.ValueString()), nil, &got); err != nil {
			resp.Diagnostics.AddError("Get member failed", err.Error())
			return
		}
		applyMember(&got, &data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	wanted := data.Email.ValueString()
	found, err := lookupByEmail(ctx, d.c, wanted)
	if err != nil {
		resp.Diagnostics.AddError("List members failed", err.Error())
		return
	}
	if found == nil {
		resp.Diagnostics.AddError("Member not found", fmt.Sprintf("No member with email %q.", wanted))
		return
	}
	applyMember(found, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func applyMember(m *member, dst *memberDSModel) {
	dst.ID = types.StringValue(m.MemberID)
	dst.Email = types.StringValue(m.Email)
	dst.FirstName = types.StringValue(m.FirstName)
	dst.LastName = types.StringValue(m.LastName)
	dst.MemberType = types.StringValue(m.MemberType)
	dst.UserKind = types.StringValue(m.UserKind)
}
