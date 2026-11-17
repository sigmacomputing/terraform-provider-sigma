package templates

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*templateDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*templateDataSource)(nil)
)

func NewTemplateDataSource() datasource.DataSource { return &templateDataSource{} }

type templateDataSource struct{ c *client.Client }

type templateDSModel struct {
	TemplateID    types.String  `tfsdk:"template_id"`
	TemplateURLID types.String  `tfsdk:"template_url_id"`
	Name          types.String  `tfsdk:"name"`
	URL           types.String  `tfsdk:"url"`
	Path          types.String  `tfsdk:"path"`
	LatestVersion types.Float64 `tfsdk:"latest_version"`
	CreatedBy     types.String  `tfsdk:"created_by"`
	UpdatedBy     types.String  `tfsdk:"updated_by"`
	CreatedAt     types.String  `tfsdk:"created_at"`
	UpdatedAt     types.String  `tfsdk:"updated_at"`
}

func (d *templateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_template"
}

func (d *templateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a single workbook template by ID via `GET /v2/templates/{templateId}`.",
		Attributes: map[string]schema.Attribute{
			"template_id":     schema.StringAttribute{Required: true},
			"template_url_id": schema.StringAttribute{Computed: true},
			"name":            schema.StringAttribute{Computed: true},
			"url":             schema.StringAttribute{Computed: true},
			"path":            schema.StringAttribute{Computed: true},
			"latest_version":  schema.Float64Attribute{Computed: true},
			"created_by":      schema.StringAttribute{Computed: true},
			"updated_by":      schema.StringAttribute{Computed: true},
			"created_at":      schema.StringAttribute{Computed: true},
			"updated_at":      schema.StringAttribute{Computed: true},
		},
	}
}

func (d *templateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *templateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var cfg templateDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got apiTemplate
	if err := d.c.Do(ctx, "GET", "/v2/templates/"+client.EscapeID(cfg.TemplateID.ValueString()), nil, &got); err != nil {
		resp.Diagnostics.AddError("Get template failed", err.Error())
		return
	}
	m := apiTemplateToModel(got)
	cfg.TemplateURLID = m.TemplateURLID
	cfg.Name = m.Name
	cfg.URL = m.URL
	cfg.Path = m.Path
	cfg.LatestVersion = m.LatestVersion
	cfg.CreatedBy = m.CreatedBy
	cfg.UpdatedBy = m.UpdatedBy
	cfg.CreatedAt = m.CreatedAt
	cfg.UpdatedAt = m.UpdatedAt
	resp.Diagnostics.Append(resp.State.Set(ctx, &cfg)...)
}
