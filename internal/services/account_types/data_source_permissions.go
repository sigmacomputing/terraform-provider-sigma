package account_types

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*permissionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*permissionsDataSource)(nil)
)

func NewPermissionsDataSource() datasource.DataSource { return &permissionsDataSource{} }

type permissionsDataSource struct{ c *client.Client }

type permissionsDSModel struct {
	AccountTypeID types.String `tfsdk:"account_type_id"`
	Permissions   types.Set    `tfsdk:"permissions"`
}

type permission struct {
	Permission  string `json:"permission"`
	Description string `json:"description"`
}

func (d *permissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_type_permissions"
}

func (d *permissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List the available permission names for a given account type. Pass these to `sigma_account_type.permissions`.",
		Attributes: map[string]schema.Attribute{
			"account_type_id": schema.StringAttribute{Required: true},
			"permissions": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (d *permissionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *permissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data permissionsDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := client.ListAllArray[permission](ctx, d.c, "/v2/accountTypes/"+client.EscapeID(data.AccountTypeID.ValueString())+"/permissions")
	if err != nil {
		resp.Diagnostics.AddError("List permissions failed", err.Error())
		return
	}
	// The API can repeat a permission name; a set rejects duplicates.
	seen := make(map[string]struct{}, len(all))
	names := make([]string, 0, len(all))
	for _, p := range all {
		if _, dup := seen[p.Permission]; dup {
			continue
		}
		seen[p.Permission] = struct{}{}
		names = append(names, p.Permission)
	}
	set, diags := types.SetValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = set
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
