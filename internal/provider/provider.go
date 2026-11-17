// Package provider wires the Sigma Terraform provider: schema, Configure,
// and the lists of resources and data sources exposed to Terraform.
package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/account_types"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/allowed_ips"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/apiconnectors"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/connections"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/credentials"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/deploymentpolicies"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/folders"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/jwks"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/members"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/orgsettings"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/plugins"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/saml"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/shared_templates"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/smtp"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/sourceswap"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/storage_integrations"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/tags"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/teams"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/templates"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/tenants"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/user_attributes"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/whoami"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/workbooks"
	"github.com/sigmacomputing/terraform-provider-sigma/internal/services/workspaces"
)

// Ensure SigmaProvider satisfies the framework's Provider interface.
var _ provider.Provider = (*SigmaProvider)(nil)

// SigmaProvider is the root provider type.
type SigmaProvider struct {
	version string
}

// New returns a Provider factory bound to a build version (set at link time
// via -ldflags or in main.go).
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &SigmaProvider{version: version}
	}
}

func (p *SigmaProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "sigma"
	resp.Version = p.version
}

// providerModel mirrors the provider schema for Configure().
type providerModel struct {
	BaseURL      types.String `tfsdk:"base_url"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

func (p *SigmaProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage a Sigma Computing organization as code: members, teams, connections, user attributes, account types, and API credentials.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional:    true,
				Description: "Sigma API base URL. Defaults to " + client.DefaultBaseURL + ". Override per region. Falls back to SIGMA_BASE_URL.",
			},
			"client_id": schema.StringAttribute{
				Optional:    true,
				Description: "OAuth2 client ID (from Sigma admin → Developer Access). Falls back to SIGMA_CLIENT_ID.",
			},
			"client_secret": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "OAuth2 client secret. Falls back to SIGMA_CLIENT_SECRET. Never written to state or logs.",
			},
		},
	}
}

func (p *SigmaProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data providerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	baseURL := stringOrEnv(data.BaseURL, "SIGMA_BASE_URL", client.DefaultBaseURL)
	clientID := stringOrEnv(data.ClientID, "SIGMA_CLIENT_ID", "")
	clientSecret := stringOrEnv(data.ClientSecret, "SIGMA_CLIENT_SECRET", "")

	if clientID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Missing client_id",
			"Set the provider attribute `client_id` or the environment variable SIGMA_CLIENT_ID.",
		)
	}
	if clientSecret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_secret"),
			"Missing client_secret",
			"Set the provider attribute `client_secret` or the environment variable SIGMA_CLIENT_SECRET.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	c, err := client.New(ctx, client.Config{
		BaseURL:      baseURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		UserAgent:    "terraform-provider-sigma/" + p.version,
	})
	if err != nil {
		resp.Diagnostics.AddError("sigma client init failed", err.Error())
		return
	}

	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *SigmaProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		// Phase 1
		members.NewResource,
		members.NewMemberAccountTypeResource,
		teams.NewTeamResource,
		teams.NewTeamMembershipResource,
		account_types.NewResource,
		user_attributes.NewResource,
		user_attributes.NewUserAssignmentResource,
		user_attributes.NewTeamAssignmentResource,
		connections.NewResource,
		connections.NewGrantResource,
		connections.NewPathGrantResource,
		credentials.NewResource,
		// Phase 2 (verified end-to-end against AWS US-West sandbox)
		workspaces.NewResource,
		workspaces.NewGrantResource,
		tags.NewResource,
		folders.NewResource,
		tenants.NewResource,
		sourceswap.NewResource,
		// Org admin + content-deployment helpers
		allowed_ips.NewResource,
		shared_templates.NewResource,
		apiconnectors.NewCredentialResource,
		apiconnectors.NewConnectorResource,
		apiconnectors.NewConnectorGrantResource,
		plugins.NewResource,
		orgsettings.NewResource,
		deploymentpolicies.NewResource,
		deploymentpolicies.NewFileResource,
		deploymentpolicies.NewTenantResource,
		smtp.NewResource,
		storage_integrations.NewResource,
		user_attributes.NewTenantAssignmentResource,
		tenants.NewDeploymentCapabilityResource,
		saml.NewCertificateResource,
		jwks.NewSetResource,
		jwks.NewKeyResource,
	}
}

func (p *SigmaProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		// Phase 1
		members.NewMembersDataSource,
		members.NewMemberDataSource,
		members.NewMemberTeamsDataSource,
		teams.NewTeamsDataSource,
		teams.NewTeamUserAttributesDataSource,
		account_types.NewPermissionsDataSource,
		account_types.NewAccountTypesDataSource,
		connections.NewConnectionsDataSource,
		connections.NewConnectionDataSource,
		connections.NewPathDataSource,
		connections.NewTableColumnsDataSource,
		// Phase 2 (verified end-to-end against AWS US-West sandbox)
		workspaces.NewWorkspacesDataSource,
		tags.NewTagsDataSource,
		tags.NewTagWorkbooksDataSource,
		workbooks.NewWorkbookDataSource,
		workbooks.NewWorkbooksDataSource,
		allowed_ips.NewDataSource,
		shared_templates.NewDataSource,
		apiconnectors.NewConnectorsDataSource,
		apiconnectors.NewCredentialsDataSource,
		templates.NewTemplatesDataSource,
		templates.NewTemplateDataSource,
		plugins.NewPluginsDataSource,
		deploymentpolicies.NewTenantsDataSource,
		storage_integrations.NewDataSource,
		saml.NewServiceProvidersDataSource,
		whoami.NewDataSource,
	}
}

// stringOrEnv resolves a provider config value with fallback chain:
//
//	explicit (non-null, non-unknown) -> env var -> default.
func stringOrEnv(v types.String, envKey, def string) string {
	if !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		return v.ValueString()
	}
	if e := os.Getenv(envKey); e != "" {
		return e
	}
	return def
}
