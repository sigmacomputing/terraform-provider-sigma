package plugins

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
	_ datasource.DataSource              = (*pluginsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*pluginsDataSource)(nil)
)

func NewPluginsDataSource() datasource.DataSource { return &pluginsDataSource{} }

type pluginsDataSource struct{ c *client.Client }

type pluginsModel struct {
	Entries types.List `tfsdk:"entries"`
}

type pluginEntryModel struct {
	PluginID    types.String `tfsdk:"plugin_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	URL         types.String `tfsdk:"url"`
	DevURL      types.String `tfsdk:"dev_url"`
	Type        types.String `tfsdk:"type"`
}

var pluginEntryAttrTypes = map[string]attr.Type{
	"plugin_id":   types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"url":         types.StringType,
	"dev_url":     types.StringType,
	"type":        types.StringType,
}

func (d *pluginsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugins"
}

func (d *pluginsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists every custom plugin in the org (Sigma beta API).",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"plugin_id":   schema.StringAttribute{Computed: true},
						"name":        schema.StringAttribute{Computed: true},
						"description": schema.StringAttribute{Computed: true},
						"url":         schema.StringAttribute{Computed: true},
						"dev_url":     schema.StringAttribute{Computed: true},
						"type":        schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *pluginsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *pluginsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	entries, err := client.ListAllPageToken[plugin](ctx, d.c, "/v2/plugins", client.PageSize(client.MaxPageSize))
	if err != nil {
		resp.Diagnostics.AddError("List plugins failed", err.Error())
		return
	}
	models := make([]pluginEntryModel, 0, len(entries))
	for _, p := range entries {
		models = append(models, pluginEntryModel{
			PluginID:    types.StringValue(p.PluginID),
			Name:        types.StringValue(p.Name),
			Description: strOrNull(p.Description),
			URL:         types.StringValue(p.URL),
			DevURL:      strOrNull(p.DevURL),
			Type:        types.StringValue(p.Type),
		})
	}
	list, d2 := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: pluginEntryAttrTypes}, models)
	resp.Diagnostics.Append(d2...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &pluginsModel{Entries: list})...)
}
