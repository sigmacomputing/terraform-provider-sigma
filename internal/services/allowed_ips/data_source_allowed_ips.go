package allowed_ips

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
	_ datasource.DataSource              = (*allowedIPsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*allowedIPsDataSource)(nil)
)

func NewDataSource() datasource.DataSource { return &allowedIPsDataSource{} }

type allowedIPsDataSource struct{ c *client.Client }

type allowedIPsModel struct {
	Entries types.List `tfsdk:"entries"`
}

type allowedIPEntryModel struct {
	IPAllowlistEntryID types.String `tfsdk:"ip_allowlist_entry_id"`
	IP                 types.String `tfsdk:"ip"`
	Scope              types.String `tfsdk:"scope"`
	Description        types.String `tfsdk:"description"`
}

func (d *allowedIPsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allowed_ips"
}

func (d *allowedIPsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every entry in the org's IP allowlist (Sigma beta API).",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip_allowlist_entry_id": schema.StringAttribute{Computed: true},
						"ip":                    schema.StringAttribute{Computed: true},
						"scope":                 schema.StringAttribute{Computed: true},
						"description":           schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *allowedIPsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *allowedIPsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	entries, err := client.ListAllPageToken[allowedIPEntry](ctx, d.c, "/v3alpha/allowedIps", client.PageSize(client.MaxPageSize))
	if err != nil {
		resp.Diagnostics.AddError("List allowed IPs failed", err.Error())
		return
	}
	models := make([]allowedIPEntryModel, 0, len(entries))
	for _, e := range entries {
		desc := types.StringNull()
		if e.Description != "" {
			desc = types.StringValue(e.Description)
		}
		models = append(models, allowedIPEntryModel{
			IPAllowlistEntryID: types.StringValue(e.IPAllowlistEntryID),
			IP:                 types.StringValue(e.IP),
			Scope:              types.StringValue(e.Scope),
			Description:        desc,
		})
	}
	t := types.ObjectType{AttrTypes: map[string]attr.Type{
		"ip_allowlist_entry_id": types.StringType,
		"ip":                    types.StringType,
		"scope":                 types.StringType,
		"description":           types.StringType,
	}}
	list, d2 := types.ListValueFrom(ctx, t, models)
	resp.Diagnostics.Append(d2...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &allowedIPsModel{Entries: list})...)
}
