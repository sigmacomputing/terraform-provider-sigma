package storage_integrations

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

const listKey = "storage-integrations"

var (
	_ datasource.DataSource              = (*storageIntegrationsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*storageIntegrationsDataSource)(nil)
)

func NewDataSource() datasource.DataSource { return &storageIntegrationsDataSource{} }

type storageIntegrationsDataSource struct{ c *client.Client }

type storageIntegrationsModel struct {
	Entries types.List `tfsdk:"entries"`
}

var entryAttrTypes = map[string]attr.Type{
	"id":                   types.StringType,
	"provider_type":        types.StringType,
	"name":                 types.StringType,
	"allowed_location":     types.StringType,
	"storage_aws_role_arn": types.StringType,
	"external_id":          types.StringType,
	"tenant_id":            types.StringType,
}

func listAll(ctx context.Context, c *client.Client) ([]storageIntegrationResp, error) {
	return client.CachedList(c, listKey, func() ([]storageIntegrationResp, error) {
		var out struct {
			Entries []storageIntegrationResp `json:"entries"`
		}
		if err := c.Do(ctx, "GET", basePath, nil, &out); err != nil {
			return nil, err
		}
		return out.Entries, nil
	})
}

func (d *storageIntegrationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_integrations"
}

func (d *storageIntegrationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	str := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc}
	}
	resp.Schema = schema.Schema{
		Description: "Lists the organization's storage integrations.",
		Attributes: map[string]schema.Attribute{
			"entries": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":                   str("Storage integration ID."),
					"provider_type":        str("`s3`, `gcs` or `azure`."),
					"name":                 str("Display name."),
					"allowed_location":     str("Allowed storage location."),
					"storage_aws_role_arn": str("S3 role ARN, if any."),
					"external_id":          str("S3 trust-policy external ID, if any."),
					"tenant_id":            str("Azure tenant ID, if any."),
				}},
			},
		},
	}
}

func (d *storageIntegrationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *storageIntegrationsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := listAll(ctx, d.c)
	if err != nil {
		resp.Diagnostics.AddError("List storage integrations failed", err.Error())
		return
	}
	objs := make([]attr.Value, 0, len(all))
	for _, e := range all {
		o, diags := types.ObjectValue(entryAttrTypes, map[string]attr.Value{
			"id":                   types.StringValue(e.StorageIntegrationID),
			"provider_type":        types.StringValue(e.Provider),
			"name":                 strOrNull(e.Name),
			"allowed_location":     types.StringValue(e.AllowedLocation),
			"storage_aws_role_arn": strOrNull(e.StorageAwsRoleArn),
			"external_id":          strOrNull(e.ExternalID),
			"tenant_id":            strOrNull(e.TenantID),
		})
		resp.Diagnostics.Append(diags...)
		objs = append(objs, o)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: entryAttrTypes}, objs)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &storageIntegrationsModel{Entries: list})...)
}
