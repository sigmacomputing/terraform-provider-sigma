package apiconnectors

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*credentialsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*credentialsDataSource)(nil)
)

func NewCredentialsDataSource() datasource.DataSource { return &credentialsDataSource{} }

type credentialsDataSource struct{ c *client.Client }

type credentialsDSModel struct {
	Credentials []credentialDSEntry `tfsdk:"credentials"`
}
type credentialDSEntry struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	AuthMethod  types.String `tfsdk:"auth_method"`
	Description types.String `tfsdk:"description"`
}

func (d *credentialsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_connector_credentials"
}
func (d *credentialsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "List all API-connector credentials. Secrets are never returned.",
		Attributes: map[string]schema.Attribute{
			"credentials": schema.ListNestedAttribute{Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"id":          schema.StringAttribute{Computed: true},
					"name":        schema.StringAttribute{Computed: true},
					"auth_method": schema.StringAttribute{Computed: true},
					"description": schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}
func (d *credentialsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
func (d *credentialsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	all, err := client.ListAllPageToken[apiCredential](ctx, d.c, "/v2/api-credentials", client.PageSize(client.MaxPageSize))
	if err != nil {
		resp.Diagnostics.AddError("List API connector credentials failed", err.Error())
		return
	}
	out := credentialsDSModel{Credentials: make([]credentialDSEntry, 0, len(all))}
	for _, c := range all {
		out.Credentials = append(out.Credentials, credentialDSEntry{
			ID:          types.StringValue(c.APICredentialID),
			Name:        types.StringValue(c.Name),
			AuthMethod:  types.StringValue(c.AuthMethod),
			Description: types.StringValue(c.Description),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
