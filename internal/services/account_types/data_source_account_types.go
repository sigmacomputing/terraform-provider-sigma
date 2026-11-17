package account_types

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*accountTypesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*accountTypesDataSource)(nil)
)

func NewAccountTypesDataSource() datasource.DataSource { return &accountTypesDataSource{} }

type accountTypesDataSource struct{ c *client.Client }

type accountTypesModel struct {
	Entries types.List `tfsdk:"entries"`
}

var accountTypeAttrTypes = map[string]attr.Type{
	"id":           types.StringType,
	"name":         types.StringType,
	"description":  types.StringType,
	"is_custom":    types.BoolType,
	"license_type": types.StringType,
}

func (d *accountTypesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_types"
}

func (d *accountTypesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all account types in the organization, built-in and custom. Use it to look up ids of built-in types, which `sigma_account_type` can't manage.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":           schema.StringAttribute{Computed: true, Description: "Account type ID."},
					"name":         schema.StringAttribute{Computed: true, Description: "Account type name."},
					"description":  schema.StringAttribute{Computed: true, Description: "Description."},
					"is_custom":    schema.BoolAttribute{Computed: true, Description: "Whether the type was created by the organization."},
					"license_type": schema.StringAttribute{Computed: true, Description: "License type, e.g. `build`, `act`, `view`."},
				}},
			},
		},
	}
}

func (d *accountTypesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *accountTypesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := listAccountTypes(ctx, d.c)
	if err != nil {
		resp.Diagnostics.AddError("List account types failed", err.Error())
		return
	}
	objs := make([]attr.Value, 0, len(all))
	for _, t := range all {
		o, diags := types.ObjectValue(accountTypeAttrTypes, map[string]attr.Value{
			"id":           types.StringValue(t.AccountTypeID),
			"name":         types.StringValue(t.AccountTypeName),
			"description":  types.StringValue(t.Description),
			"is_custom":    types.BoolValue(t.IsCustom),
			"license_type": types.StringValue(t.LicenseType),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, o)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: accountTypeAttrTypes}, objs)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &accountTypesModel{Entries: list})...)
}
