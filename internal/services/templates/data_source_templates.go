// Package templates implements the sigma_templates / sigma_template data sources.
//
// Templates are first-class workbook templates managed via /v2/templates
// (distinct from cross-org `sigma_shared_templates` which are imports of
// templates from another org). The four endpoints are:
//
//	GET  /v2/templates                          → list (paginated)
//	GET  /v2/templates/{templateId}             → one
//	POST /v2/templates/save_workbook            → create workbook from template
//	POST /v2/templates/{templateId}/swapSources → one-shot source swap
package templates

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
	_ datasource.DataSource              = (*templatesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*templatesDataSource)(nil)
)

func NewTemplatesDataSource() datasource.DataSource { return &templatesDataSource{} }

type templatesDataSource struct{ c *client.Client }

type templatesDSModel struct {
	Entries types.List `tfsdk:"entries"`
}

type apiTemplate struct {
	TemplateID    string  `json:"templateId"`
	TemplateURLID string  `json:"templateUrlId"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	Path          string  `json:"path"`
	LatestVersion float64 `json:"latestVersion"`
	CreatedBy     string  `json:"createdBy,omitempty"`
	UpdatedBy     string  `json:"updatedBy,omitempty"`
	CreatedAt     string  `json:"createdAt,omitempty"`
	UpdatedAt     string  `json:"updatedAt,omitempty"`
}

type templateEntryModel struct {
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

func templateAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"template_id":     types.StringType,
		"template_url_id": types.StringType,
		"name":            types.StringType,
		"url":             types.StringType,
		"path":            types.StringType,
		"latest_version":  types.Float64Type,
		"created_by":      types.StringType,
		"updated_by":      types.StringType,
		"created_at":      types.StringType,
		"updated_at":      types.StringType,
	}
}

func (d *templatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_templates"
}

func (d *templatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all workbook templates in the org via `GET /v2/templates`.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"template_id":     schema.StringAttribute{Computed: true},
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
				},
			},
		},
	}
}

func (d *templatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *templatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	entries, err := client.ListAllPaged[apiTemplate](ctx, d.c, "/v2/templates", nil)
	if err != nil {
		resp.Diagnostics.AddError("List templates failed", err.Error())
		return
	}
	models := make([]templateEntryModel, 0, len(entries))
	for _, t := range entries {
		models = append(models, apiTemplateToModel(t))
	}
	t := types.ObjectType{AttrTypes: templateAttrTypes()}
	list, dErr := types.ListValueFrom(ctx, t, models)
	resp.Diagnostics.Append(dErr...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &templatesDSModel{Entries: list})...)
}

func apiTemplateToModel(t apiTemplate) templateEntryModel {
	stringOrNull := func(s string) types.String {
		if s == "" {
			return types.StringNull()
		}
		return types.StringValue(s)
	}
	return templateEntryModel{
		TemplateID:    types.StringValue(t.TemplateID),
		TemplateURLID: types.StringValue(t.TemplateURLID),
		Name:          types.StringValue(t.Name),
		URL:           types.StringValue(t.URL),
		Path:          types.StringValue(t.Path),
		LatestVersion: types.Float64Value(t.LatestVersion),
		CreatedBy:     stringOrNull(t.CreatedBy),
		UpdatedBy:     stringOrNull(t.UpdatedBy),
		CreatedAt:     stringOrNull(t.CreatedAt),
		UpdatedAt:     stringOrNull(t.UpdatedAt),
	}
}
