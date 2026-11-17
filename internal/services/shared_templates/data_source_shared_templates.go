package shared_templates

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
	_ datasource.DataSource              = (*sharedTemplatesDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*sharedTemplatesDataSource)(nil)
)

func NewDataSource() datasource.DataSource { return &sharedTemplatesDataSource{} }

type sharedTemplatesDataSource struct{ c *client.Client }

type sharedTemplatesModel struct {
	Entries types.List `tfsdk:"entries"`
}

type sharedTemplateEntry struct {
	ProviderOrganizationID string `json:"providerOrganizationId"`
	Name                   string `json:"name"`
	ShareID                string `json:"shareId"`
	CreatedAt              string `json:"createdAt"`
	Description            string `json:"description,omitempty"`
	Pending                bool   `json:"pending,omitempty"`
}

type sharedTemplateEntryModel struct {
	ProviderOrganizationID types.String `tfsdk:"provider_organization_id"`
	Name                   types.String `tfsdk:"name"`
	ShareID                types.String `tfsdk:"share_id"`
	CreatedAt              types.String `tfsdk:"created_at"`
	Description            types.String `tfsdk:"description"`
	Pending                types.Bool   `tfsdk:"pending"`
}

func (d *sharedTemplatesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_templates"
}

func (d *sharedTemplatesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists workbook templates shared with this org via `GET /v2/shared_templates/shared_with_you`. Each entry includes a `share_id` you can plug into `sigma_shared_template_acceptance` to accept and import the template.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"provider_organization_id": schema.StringAttribute{Computed: true, Description: "Org that shared the template."},
						"name":                     schema.StringAttribute{Computed: true},
						"share_id":                 schema.StringAttribute{Computed: true},
						"created_at":               schema.StringAttribute{Computed: true, Description: "When the share was created."},
						"description":              schema.StringAttribute{Computed: true},
						"pending":                  schema.BoolAttribute{Computed: true, Description: "True if not yet accepted by this org."},
					},
				},
			},
		},
	}
}

func (d *sharedTemplatesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *sharedTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	entries, err := client.ListAllPaged[sharedTemplateEntry](ctx, d.c, "/v2/shared_templates/shared_with_you", nil)
	if err != nil {
		resp.Diagnostics.AddError("List shared templates failed", err.Error())
		return
	}
	models := make([]sharedTemplateEntryModel, 0, len(entries))
	for _, e := range entries {
		desc := types.StringNull()
		if e.Description != "" {
			desc = types.StringValue(e.Description)
		}
		models = append(models, sharedTemplateEntryModel{
			ProviderOrganizationID: types.StringValue(e.ProviderOrganizationID),
			Name:                   types.StringValue(e.Name),
			ShareID:                types.StringValue(e.ShareID),
			CreatedAt:              types.StringValue(e.CreatedAt),
			Description:            desc,
			Pending:                types.BoolValue(e.Pending),
		})
	}
	t := types.ObjectType{AttrTypes: map[string]attr.Type{
		"provider_organization_id": types.StringType,
		"name":                     types.StringType,
		"share_id":                 types.StringType,
		"created_at":               types.StringType,
		"description":              types.StringType,
		"pending":                  types.BoolType,
	}}
	list, d2 := types.ListValueFrom(ctx, t, models)
	resp.Diagnostics.Append(d2...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &sharedTemplatesModel{Entries: list})...)
}
