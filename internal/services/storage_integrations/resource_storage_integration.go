// Package storage_integrations implements sigma_storage_integration.
package storage_integrations

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

const basePath = "/v2/organizations/settings/storageIntegrations"

var (
	_ resource.Resource                = (*storageIntegrationResource)(nil)
	_ resource.ResourceWithConfigure   = (*storageIntegrationResource)(nil)
	_ resource.ResourceWithImportState = (*storageIntegrationResource)(nil)
)

func NewResource() resource.Resource { return &storageIntegrationResource{} }

type storageIntegrationResource struct{ c *client.Client }

type storageIntegrationModel struct {
	ID                              types.String `tfsdk:"id"`
	Provider                        types.String `tfsdk:"provider_type"`
	Name                            types.String `tfsdk:"name"`
	BucketName                      types.String `tfsdk:"bucket_name"`
	PathPrefix                      types.String `tfsdk:"path_prefix"`
	StorageAwsRoleArn               types.String `tfsdk:"storage_aws_role_arn"`
	KmsKeyName                      types.String `tfsdk:"kms_key_name"`
	GcpServiceAccount               types.String `tfsdk:"gcp_service_account"`
	WarehouseStorageIntegrationName types.String `tfsdk:"warehouse_storage_integration_name"`
	StorageAccount                  types.String `tfsdk:"storage_account"`
	TenantID                        types.String `tfsdk:"tenant_id"`
	AllowedLocation                 types.String `tfsdk:"allowed_location"`
	ExternalID                      types.String `tfsdk:"external_id"`
}

// storageIntegrationResp is one integration as returned by the list/create/update endpoints.
type storageIntegrationResp struct {
	StorageIntegrationID string `json:"storageIntegrationId"`
	Provider             string `json:"provider"`
	AllowedLocation      string `json:"allowedLocation"`
	Name                 string `json:"name"`
	KmsKeyName           string `json:"kmsKeyName"`
	StorageAwsRoleArn    string `json:"storageAwsRoleArn"`
	ExternalID           string `json:"externalId"`
	TenantID             string `json:"tenantId"`
}

func (r *storageIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_integration"
}

func optStr(desc string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Description: desc}
}

func optComputedStr(desc string) schema.StringAttribute {
	return schema.StringAttribute{Optional: true, Computed: true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		Description:   desc}
}

func (r *storageIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Cloud storage integration (S3, GCS or Azure) used for exports and audit logs. An organization that already has one must import it: creating over an existing integration is refused because Sigma would overwrite it. Fields are provider-specific: set the ones that apply to `provider_type`. `bucket_name`, `path_prefix`, `gcp_service_account`, `warehouse_storage_integration_name` and `storage_account` are not returned by Sigma, so changes made outside Terraform to them are not detected.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"provider_type": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Storage provider: `s3`, `gcs` or `azure`. Changing it forces replacement."},
			"name":                               optComputedStr("Display name of the integration."),
			"bucket_name":                        schema.StringAttribute{Required: true, Description: "Bucket (S3/GCS) or container (Azure) name."},
			"path_prefix":                        schema.StringAttribute{Required: true, Description: "Path prefix within the bucket or container."},
			"storage_aws_role_arn":               optComputedStr("S3 only: IAM role ARN Sigma assumes."),
			"kms_key_name":                       optComputedStr("S3 only: KMS key used for encryption."),
			"gcp_service_account":                optStr("GCS only: service account used to access the bucket."),
			"warehouse_storage_integration_name": optStr("GCS only: name of the warehouse storage integration."),
			"storage_account":                    optStr("Azure only: storage account name."),
			"tenant_id":                          optComputedStr("Azure only: Azure tenant ID."),
			"allowed_location": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Storage location Sigma is allowed to write to, derived from bucket and prefix."},
			"external_id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "External ID to put in the S3 role trust policy."},
		},
	}
}

func (r *storageIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.c = c
}

// requestBody builds the provider-discriminated POST/PATCH body, omitting unset fields.
func requestBody(m *storageIntegrationModel) map[string]any {
	b := map[string]any{
		"provider":   m.Provider.ValueString(),
		"bucketName": m.BucketName.ValueString(),
		"pathPrefix": m.PathPrefix.ValueString(),
	}
	for k, v := range map[string]types.String{
		"name":                            m.Name,
		"storageAwsRoleArn":               m.StorageAwsRoleArn,
		"kmsKeyName":                      m.KmsKeyName,
		"gcpServiceAccount":               m.GcpServiceAccount,
		"warehouseStorageIntegrationName": m.WarehouseStorageIntegrationName,
		"storageAccount":                  m.StorageAccount,
		"tenantId":                        m.TenantID,
	} {
		if !v.IsNull() && !v.IsUnknown() {
			b[k] = v.ValueString()
		}
	}
	return b
}

func (r *storageIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan storageIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Sigma replaces an existing integration on POST (same ID back), and destroy would then delete it.
	var existing struct {
		Entries []storageIntegrationResp `json:"entries"`
	}
	if err := r.c.Do(ctx, "GET", basePath, nil, &existing); err != nil {
		resp.Diagnostics.AddError("List storage integrations failed", err.Error())
		return
	}
	if len(existing.Entries) > 0 {
		resp.Diagnostics.AddError("Storage integration already exists",
			fmt.Sprintf("This organization already has storage integration %q (%s). Creating another would overwrite it. Import it instead: `terraform import sigma_storage_integration.<name> %s`.",
				existing.Entries[0].Name, existing.Entries[0].StorageIntegrationID, existing.Entries[0].StorageIntegrationID))
		return
	}
	var got storageIntegrationResp
	defer client.InvalidateList(r.c, listKey)
	if err := r.c.Do(ctx, "POST", basePath, requestBody(&plan), &got); err != nil {
		resp.Diagnostics.AddError("Create storage integration failed", err.Error())
		return
	}
	plan.ID = types.StringValue(got.StorageIntegrationID)
	fill(&plan, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *storageIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state storageIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	all, err := listAll(ctx, r.c)
	if err != nil {
		resp.Diagnostics.AddError("List storage integrations failed", err.Error())
		return
	}
	for _, e := range all {
		if e.StorageIntegrationID == state.ID.ValueString() {
			fill(&state, e)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *storageIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state storageIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got storageIntegrationResp
	defer client.InvalidateList(r.c, listKey)
	if err := r.c.Do(ctx, "PATCH", basePath+"/"+client.EscapeID(state.ID.ValueString()), requestBody(&plan), &got); err != nil {
		resp.Diagnostics.AddError("Update storage integration failed", err.Error())
		return
	}
	plan.ID = state.ID
	fill(&plan, got)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *storageIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state storageIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, listKey)
	if err := r.c.Do(ctx, "DELETE", basePath+"/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete storage integration failed", err.Error())
	}
}

func (r *storageIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// bucket_name/path_prefix aren't returned; set them in config and the next apply reconciles.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// fill copies server-returned fields onto the model. Empty strings mean "not set for this provider".
func fill(m *storageIntegrationModel, got storageIntegrationResp) {
	m.Provider = types.StringValue(got.Provider)
	m.AllowedLocation = types.StringValue(got.AllowedLocation)
	m.ExternalID = strOrNull(got.ExternalID)
	setIfReturned(&m.Name, got.Name)
	setIfReturned(&m.KmsKeyName, got.KmsKeyName)
	setIfReturned(&m.StorageAwsRoleArn, got.StorageAwsRoleArn)
	setIfReturned(&m.TenantID, got.TenantID)
}

func strOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

// setIfReturned overwrites v only when Sigma returned a value; an unresolved unknown becomes null.
func setIfReturned(v *types.String, s string) {
	if s != "" {
		*v = types.StringValue(s)
	} else if v.IsUnknown() {
		*v = types.StringNull()
	}
}
