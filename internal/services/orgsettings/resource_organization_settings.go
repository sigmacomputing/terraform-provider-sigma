// Package orgsettings implements sigma_organization_settings.
package orgsettings

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

// singletonID: one settings object per org, always exists, no org ID to key on.
const singletonID = "organization"

const (
	pathAuditLogging           = "/v2/organizations/settings/auditLogging"
	pathBulkCopy               = "/v2/organizations/settings/bulkCopy"
	pathComments               = "/v2/organizations/settings/comments"
	pathCSVUpload              = "/v2/organizations/settings/csvUpload"
	pathLicenseUpgradeRequests = "/v2/organizations/settings/licenseUpgradeRequests"
	pathPublicEmbeds           = "/v2/organizations/settings/publicEmbeds"
	pathTimezone               = "/v2/organizations/settings/timezone"
	pathAiChatHistory          = "/v2/organizations/settings/aiChatHistory"
	pathEmailBranding          = "/v2/organizations/settings/emailBranding"
	pathSampleConnections      = "/v2/organizations/settings/sampleConnections"
)

var (
	_ resource.Resource                = (*organizationSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*organizationSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*organizationSettingsResource)(nil)
)

func NewResource() resource.Resource { return &organizationSettingsResource{} }

type organizationSettingsResource struct{ c *client.Client }

type organizationSettingsModel struct {
	ID                     types.String `tfsdk:"id"`
	AuditLogging           types.Object `tfsdk:"audit_logging"`
	BulkCopy               types.Object `tfsdk:"bulk_copy"`
	Comments               types.Object `tfsdk:"comments"`
	CSVUpload              types.Object `tfsdk:"csv_upload"`
	LicenseUpgradeRequests types.Object `tfsdk:"license_upgrade_requests"`
	PublicEmbeds           types.Object `tfsdk:"public_embeds"`
	Timezone               types.Object `tfsdk:"timezone"`
	AiChatHistory          types.Object `tfsdk:"ai_chat_history"`
	EmailBranding          types.Object `tfsdk:"email_branding"`
	SampleConnections      types.Object `tfsdk:"sample_connections"`
}

// enabledModel: shared shape for bulk_copy, license_upgrade_requests, public_embeds.
type enabledModel struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

var enabledAttrTypes = map[string]attr.Type{"enabled": types.BoolType}

// GET returns `status` (enum); PATCH takes `enabled` (bool), derived from status.
type auditLoggingModel struct {
	Enabled types.Bool   `tfsdk:"enabled"`
	Status  types.String `tfsdk:"status"`
}

var auditLoggingAttrTypes = map[string]attr.Type{
	"enabled": types.BoolType,
	"status":  types.StringType,
}

type commentsModel struct {
	Enabled                 types.Bool  `tfsdk:"enabled"`
	AllowAnnotations        types.Bool  `tfsdk:"allow_annotations"`
	AnnotationRetentionDays types.Int64 `tfsdk:"annotation_retention_days"`
}

var commentsAttrTypes = map[string]attr.Type{
	"enabled":                   types.BoolType,
	"allow_annotations":         types.BoolType,
	"annotation_retention_days": types.Int64Type,
}

type csvUploadModel struct {
	Enabled               types.Bool `tfsdk:"enabled"`
	UseInternalStages     types.Bool `tfsdk:"use_internal_stages"`
	UseStorageIntegration types.Bool `tfsdk:"use_storage_integration"`
	IncreaseUploadLimit   types.Bool `tfsdk:"increase_upload_limit"`
}

var csvUploadAttrTypes = map[string]attr.Type{
	"enabled":                 types.BoolType,
	"use_internal_stages":     types.BoolType,
	"use_storage_integration": types.BoolType,
	"increase_upload_limit":   types.BoolType,
}

type timezoneModel struct {
	Timezone types.String `tfsdk:"timezone"`
}

var timezoneAttrTypes = map[string]attr.Type{"timezone": types.StringType}

type aiChatHistoryModel struct {
	Enabled         types.Bool   `tfsdk:"enabled"`
	RetentionPeriod types.String `tfsdk:"retention_period"`
}

var aiChatHistoryAttrTypes = map[string]attr.Type{
	"enabled":          types.BoolType,
	"retention_period": types.StringType,
}

type emailBrandingModel struct {
	DisplayName        types.String `tfsdk:"display_name"`
	SignatureName      types.String `tfsdk:"signature_name"`
	ReplyToAddress     types.String `tfsdk:"reply_to_address"`
	HideSenderInfo     types.Bool   `tfsdk:"hide_sender_info"`
	GlobalBccAddresses types.List   `tfsdk:"global_bcc_addresses"`
	FooterBody         types.String `tfsdk:"footer_body"`
}

var emailBrandingAttrTypes = map[string]attr.Type{
	"display_name":         types.StringType,
	"signature_name":       types.StringType,
	"reply_to_address":     types.StringType,
	"hide_sender_info":     types.BoolType,
	"global_bcc_addresses": types.ListType{ElemType: types.StringType},
	"footer_body":          types.StringType,
}

type sampleConnectionsModel struct {
	EnableSnowflake  types.Bool `tfsdk:"enable_snowflake"`
	EnableDatabricks types.Bool `tfsdk:"enable_databricks"`
}

var sampleConnectionsAttrTypes = map[string]attr.Type{
	"enable_snowflake":  types.BoolType,
	"enable_databricks": types.BoolType,
}

type auditLoggingResp struct {
	Status string `json:"status"`
}
type auditLoggingPatch struct {
	Enabled bool `json:"enabled"`
}

type enabledResp struct {
	Enabled bool `json:"enabled"`
}
type enabledPatch = enabledResp

type commentsResp struct {
	Enabled                 bool `json:"enabled"`
	AllowAnnotations        bool `json:"allowAnnotations"`
	AnnotationRetentionDays int  `json:"annotationRetentionDays"`
}
type commentsPatch = commentsResp

type csvUploadResp struct {
	Enabled               bool `json:"enabled"`
	UseInternalStages     bool `json:"useInternalStages"`
	UseStorageIntegration bool `json:"useStorageIntegration"`
	IncreaseUploadLimit   bool `json:"increaseUploadLimit"`
}
type csvUploadPatch = csvUploadResp

type timezoneResp struct {
	Timezone string `json:"timezone"`
}
type timezonePatch = timezoneResp

type aiChatHistoryResp struct {
	Enabled         bool    `json:"enabled"`
	RetentionPeriod *string `json:"retentionPeriod"`
}
type aiChatHistoryPatch struct {
	Enabled         bool    `json:"enabled"`
	RetentionPeriod *string `json:"retentionPeriod"`
}

type emailBrandingResp struct {
	DisplayName        string   `json:"displayName"`
	SignatureName      string   `json:"signatureName"`
	ReplyToAddress     *string  `json:"replyToAddress"`
	HideSenderInfo     bool     `json:"hideSenderInfo"`
	GlobalBccAddresses []string `json:"globalBccAddresses"`
	FooterBody         string   `json:"footerBody"`
}
type emailBrandingPatch = emailBrandingResp

type sampleConnectionsResp struct {
	EnableSnowflake  bool `json:"enableSnowflake"`
	EnableDatabricks bool `json:"enableDatabricks"`
}
type sampleConnectionsPatch = sampleConnectionsResp

func (r *organizationSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_settings"
}

func (r *organizationSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Singleton resource managing a Sigma organization's settings. Each block is independently optional so a subset can be adopted piecemeal; unconfigured blocks reflect the organization's current values instead of erroring or drifting. Deleting this resource only removes it from Terraform state — Sigma has no API to delete or reset organization settings, so the org's actual settings are left exactly as last applied.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"audit_logging": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "Organization audit logging.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
					"status": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						Description: "Raw server status: `enabled`, `enable_in_progress`, `disabled`, or `disable_in_progress`. `enable_in_progress`/`disable_in_progress` mean a background task is still converging."},
				},
			},
			"bulk_copy": enabledBlockSchema("Whether bulk-copy is enabled for the organization."),
			"comments": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "Commenting on documents.",
				Attributes: map[string]schema.Attribute{
					"enabled":           schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
					"allow_annotations": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: "Whether users can capture images and annotate them in comments. Only takes effect when `enabled` is true."},
					"annotation_retention_days": schema.Int64Attribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
						Description: "How long annotation images are kept, in days. Must be between 1 and 730."},
				},
			},
			"csv_upload": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "CSV file upload.",
				Attributes: map[string]schema.Attribute{
					"enabled":             schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
					"use_internal_stages": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: "Whether CSV uploads use Snowflake internal stages. Requires an active Snowflake connection."},
					"use_storage_integration": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
						Description: "Whether CSV uploads use the organization's external storage integration instead of Sigma-managed storage."},
					"increase_upload_limit": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: "Whether the CSV upload size limit is increased for the organization."},
				},
			},
			"license_upgrade_requests": enabledBlockSchema("Whether members can request license upgrades."),
			"public_embeds":            enabledBlockSchema("Whether public embeds are enabled for the organization."),
			"timezone": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"timezone": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						Description: "IANA timezone identifier for the organization, e.g. `America/Los_Angeles`."},
				},
			},
			"ai_chat_history": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "Sigma AI chat history retention.",
				Attributes: map[string]schema.Attribute{
					"enabled": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
					"retention_period": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
						Description: "Days a chat conversation is kept: `30`, `60`, `90`, or `never`. Null if no retention period is configured."},
				},
			},
			"email_branding": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Description:   "Branding for outgoing emails. Sigma also exposes a DELETE on this endpoint to reset it; this resource doesn't call it, consistent with the resource having no delete/reset semantics overall.",
				Attributes: map[string]schema.Attribute{
					"display_name":     schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Sender display name on outgoing branded emails."},
					"signature_name":   schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Name shown in the signature block of outgoing branded emails."},
					"reply_to_address": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Reply-to address for outgoing branded emails. Null if no override is configured."},
					"hide_sender_info": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: "Whether to hide the sender's name and email address on outgoing branded emails."},
					"global_bcc_addresses": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType, PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
						Description: "Addresses BCC'd on every outgoing branded email."},
					"footer_body": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}, Description: "Footer text appended to outgoing branded emails."},
				},
			},
			"sample_connections": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"enable_snowflake":  schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: "Enable the Snowflake sample connection, if one exists for the organization."},
					"enable_databricks": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: "Enable the Databricks sample connection, if one exists for the organization."},
				},
			},
		},
	}
}

func enabledBlockSchema(desc string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:      true,
		Computed:      true,
		PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
		Attributes: map[string]schema.Attribute{
			"enabled": schema.BoolAttribute{Optional: true, Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}, Description: desc},
		},
	}
}

func (r *organizationSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *organizationSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan organizationSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.applyConfigured(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	final, diags := r.readAll(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &final)...)
}

func (r *organizationSettingsResource) Read(ctx context.Context, _ resource.ReadRequest, resp *resource.ReadResponse) {
	final, diags := r.readAll(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &final)...)
}

func (r *organizationSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state organizationSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(r.applyChanged(ctx, &plan, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	final, diags := r.readAll(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &final)...)
}

// No-op: Sigma has no delete/reset API; this only drops the resource from state.
func (r *organizationSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *organizationSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyConfigured PATCHes every block set in plan; used by Create (no prior state).
func (r *organizationSettingsResource) applyConfigured(ctx context.Context, plan *organizationSettingsModel) diag.Diagnostics {
	var diags diag.Diagnostics

	if isConfigured(plan.AuditLogging) {
		var m auditLoggingModel
		diags.Append(plan.AuditLogging.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		if err := r.c.Do(ctx, "PATCH", pathAuditLogging, auditLoggingPatch{Enabled: m.Enabled.ValueBool()}, nil); err != nil {
			diags.AddError("Update audit logging setting failed", err.Error())
			return diags
		}
	}
	if isConfigured(plan.BulkCopy) {
		diags.Append(patchEnabledBlock(ctx, r.c, pathBulkCopy, plan.BulkCopy)...)
		if diags.HasError() {
			return diags
		}
	}
	if isConfigured(plan.Comments) {
		var m commentsModel
		diags.Append(plan.Comments.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		body := commentsPatch{Enabled: m.Enabled.ValueBool(), AllowAnnotations: m.AllowAnnotations.ValueBool(), AnnotationRetentionDays: int(m.AnnotationRetentionDays.ValueInt64())}
		if err := r.c.Do(ctx, "PATCH", pathComments, body, nil); err != nil {
			diags.AddError("Update comments setting failed", err.Error())
			return diags
		}
	}
	if isConfigured(plan.CSVUpload) {
		var m csvUploadModel
		diags.Append(plan.CSVUpload.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		body := csvUploadPatch{
			Enabled:               m.Enabled.ValueBool(),
			UseInternalStages:     m.UseInternalStages.ValueBool(),
			UseStorageIntegration: m.UseStorageIntegration.ValueBool(),
			IncreaseUploadLimit:   m.IncreaseUploadLimit.ValueBool(),
		}
		if err := r.c.Do(ctx, "PATCH", pathCSVUpload, body, nil); err != nil {
			diags.AddError("Update CSV upload setting failed", err.Error())
			return diags
		}
	}
	if isConfigured(plan.LicenseUpgradeRequests) {
		diags.Append(patchEnabledBlock(ctx, r.c, pathLicenseUpgradeRequests, plan.LicenseUpgradeRequests)...)
		if diags.HasError() {
			return diags
		}
	}
	if isConfigured(plan.PublicEmbeds) {
		diags.Append(patchEnabledBlock(ctx, r.c, pathPublicEmbeds, plan.PublicEmbeds)...)
		if diags.HasError() {
			return diags
		}
	}
	if isConfigured(plan.Timezone) {
		var m timezoneModel
		diags.Append(plan.Timezone.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		if err := r.c.Do(ctx, "PATCH", pathTimezone, timezonePatch{Timezone: m.Timezone.ValueString()}, nil); err != nil {
			diags.AddError("Update timezone setting failed", err.Error())
			return diags
		}
	}
	if isConfigured(plan.AiChatHistory) {
		var m aiChatHistoryModel
		diags.Append(plan.AiChatHistory.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		body := aiChatHistoryPatch{Enabled: m.Enabled.ValueBool()}
		if !m.RetentionPeriod.IsNull() {
			v := m.RetentionPeriod.ValueString()
			body.RetentionPeriod = &v
		}
		if err := r.c.Do(ctx, "PATCH", pathAiChatHistory, body, nil); err != nil {
			diags.AddError("Update AI chat history setting failed", err.Error())
			return diags
		}
	}
	if isConfigured(plan.EmailBranding) {
		var m emailBrandingModel
		diags.Append(plan.EmailBranding.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		var bcc []string
		diags.Append(m.GlobalBccAddresses.ElementsAs(ctx, &bcc, false)...)
		if diags.HasError() {
			return diags
		}
		body := emailBrandingPatch{
			DisplayName:        m.DisplayName.ValueString(),
			SignatureName:      m.SignatureName.ValueString(),
			HideSenderInfo:     m.HideSenderInfo.ValueBool(),
			GlobalBccAddresses: bcc,
			FooterBody:         m.FooterBody.ValueString(),
		}
		if !m.ReplyToAddress.IsNull() {
			v := m.ReplyToAddress.ValueString()
			body.ReplyToAddress = &v
		}
		if err := r.c.Do(ctx, "PATCH", pathEmailBranding, body, nil); err != nil {
			diags.AddError("Update email branding setting failed", err.Error())
			return diags
		}
	}
	if isConfigured(plan.SampleConnections) {
		var m sampleConnectionsModel
		diags.Append(plan.SampleConnections.As(ctx, &m, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return diags
		}
		body := sampleConnectionsPatch{EnableSnowflake: m.EnableSnowflake.ValueBool(), EnableDatabricks: m.EnableDatabricks.ValueBool()}
		if err := r.c.Do(ctx, "PATCH", pathSampleConnections, body, nil); err != nil {
			diags.AddError("Update sample connections setting failed", err.Error())
			return diags
		}
	}
	return diags
}

// applyChanged PATCHes only blocks that differ from state; used by Update.
func (r *organizationSettingsResource) applyChanged(ctx context.Context, plan, state *organizationSettingsModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if !plan.AuditLogging.Equal(state.AuditLogging) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{AuditLogging: plan.AuditLogging})...)
	}
	if !plan.BulkCopy.Equal(state.BulkCopy) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{BulkCopy: plan.BulkCopy})...)
	}
	if !plan.Comments.Equal(state.Comments) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{Comments: plan.Comments})...)
	}
	if !plan.CSVUpload.Equal(state.CSVUpload) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{CSVUpload: plan.CSVUpload})...)
	}
	if !plan.LicenseUpgradeRequests.Equal(state.LicenseUpgradeRequests) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{LicenseUpgradeRequests: plan.LicenseUpgradeRequests})...)
	}
	if !plan.PublicEmbeds.Equal(state.PublicEmbeds) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{PublicEmbeds: plan.PublicEmbeds})...)
	}
	if !plan.Timezone.Equal(state.Timezone) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{Timezone: plan.Timezone})...)
	}
	if !plan.AiChatHistory.Equal(state.AiChatHistory) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{AiChatHistory: plan.AiChatHistory})...)
	}
	if !plan.EmailBranding.Equal(state.EmailBranding) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{EmailBranding: plan.EmailBranding})...)
	}
	if !plan.SampleConnections.Equal(state.SampleConnections) {
		diags.Append(r.applyConfigured(ctx, &organizationSettingsModel{SampleConnections: plan.SampleConnections})...)
	}
	return diags
}

// readAll GETs all 7 settings endpoints and builds the full resource state.
func (r *organizationSettingsResource) readAll(ctx context.Context) (organizationSettingsModel, diag.Diagnostics) {
	var out organizationSettingsModel
	var diags diag.Diagnostics
	out.ID = types.StringValue(singletonID)

	var al auditLoggingResp
	if err := r.c.Do(ctx, "GET", pathAuditLogging, nil, &al); err != nil {
		diags.AddError("Read audit logging setting failed", err.Error())
		return out, diags
	}
	alEnabled := al.Status == "enabled" || al.Status == "enable_in_progress"
	alObj, d := types.ObjectValue(auditLoggingAttrTypes, map[string]attr.Value{
		"enabled": types.BoolValue(alEnabled),
		"status":  types.StringValue(al.Status),
	})
	diags.Append(d...)
	out.AuditLogging = alObj

	bulkCopy, d := getEnabledBlock(ctx, r.c, pathBulkCopy)
	diags.Append(d...)
	out.BulkCopy = bulkCopy

	var cm commentsResp
	if err := r.c.Do(ctx, "GET", pathComments, nil, &cm); err != nil {
		diags.AddError("Read comments setting failed", err.Error())
		return out, diags
	}
	commentsObj, d := types.ObjectValue(commentsAttrTypes, map[string]attr.Value{
		"enabled":                   types.BoolValue(cm.Enabled),
		"allow_annotations":         types.BoolValue(cm.AllowAnnotations),
		"annotation_retention_days": types.Int64Value(int64(cm.AnnotationRetentionDays)),
	})
	diags.Append(d...)
	out.Comments = commentsObj

	var cu csvUploadResp
	if err := r.c.Do(ctx, "GET", pathCSVUpload, nil, &cu); err != nil {
		diags.AddError("Read CSV upload setting failed", err.Error())
		return out, diags
	}
	csvObj, d := types.ObjectValue(csvUploadAttrTypes, map[string]attr.Value{
		"enabled":                 types.BoolValue(cu.Enabled),
		"use_internal_stages":     types.BoolValue(cu.UseInternalStages),
		"use_storage_integration": types.BoolValue(cu.UseStorageIntegration),
		"increase_upload_limit":   types.BoolValue(cu.IncreaseUploadLimit),
	})
	diags.Append(d...)
	out.CSVUpload = csvObj

	licenseUpgrade, d := getEnabledBlock(ctx, r.c, pathLicenseUpgradeRequests)
	diags.Append(d...)
	out.LicenseUpgradeRequests = licenseUpgrade

	publicEmbeds, d := getEnabledBlock(ctx, r.c, pathPublicEmbeds)
	diags.Append(d...)
	out.PublicEmbeds = publicEmbeds

	var tz timezoneResp
	if err := r.c.Do(ctx, "GET", pathTimezone, nil, &tz); err != nil {
		diags.AddError("Read timezone setting failed", err.Error())
		return out, diags
	}
	tzObj, d := types.ObjectValue(timezoneAttrTypes, map[string]attr.Value{"timezone": types.StringValue(tz.Timezone)})
	diags.Append(d...)
	out.Timezone = tzObj

	var ach aiChatHistoryResp
	if err := r.c.Do(ctx, "GET", pathAiChatHistory, nil, &ach); err != nil {
		diags.AddError("Read AI chat history setting failed", err.Error())
		return out, diags
	}
	retentionPeriod := types.StringNull()
	if ach.RetentionPeriod != nil {
		retentionPeriod = types.StringValue(*ach.RetentionPeriod)
	}
	achObj, d := types.ObjectValue(aiChatHistoryAttrTypes, map[string]attr.Value{
		"enabled":          types.BoolValue(ach.Enabled),
		"retention_period": retentionPeriod,
	})
	diags.Append(d...)
	out.AiChatHistory = achObj

	var eb emailBrandingResp
	if err := r.c.Do(ctx, "GET", pathEmailBranding, nil, &eb); err != nil {
		diags.AddError("Read email branding setting failed", err.Error())
		return out, diags
	}
	replyToAddress := types.StringNull()
	if eb.ReplyToAddress != nil {
		replyToAddress = types.StringValue(*eb.ReplyToAddress)
	}
	bccList, d := types.ListValueFrom(ctx, types.StringType, eb.GlobalBccAddresses)
	diags.Append(d...)
	ebObj, d := types.ObjectValue(emailBrandingAttrTypes, map[string]attr.Value{
		"display_name":         types.StringValue(eb.DisplayName),
		"signature_name":       types.StringValue(eb.SignatureName),
		"reply_to_address":     replyToAddress,
		"hide_sender_info":     types.BoolValue(eb.HideSenderInfo),
		"global_bcc_addresses": bccList,
		"footer_body":          types.StringValue(eb.FooterBody),
	})
	diags.Append(d...)
	out.EmailBranding = ebObj

	var sc sampleConnectionsResp
	if err := r.c.Do(ctx, "GET", pathSampleConnections, nil, &sc); err != nil {
		diags.AddError("Read sample connections setting failed", err.Error())
		return out, diags
	}
	scObj, d := types.ObjectValue(sampleConnectionsAttrTypes, map[string]attr.Value{
		"enable_snowflake":  types.BoolValue(sc.EnableSnowflake),
		"enable_databricks": types.BoolValue(sc.EnableDatabricks),
	})
	diags.Append(d...)
	out.SampleConnections = scObj

	return out, diags
}

func getEnabledBlock(ctx context.Context, c *client.Client, p string) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	var got enabledResp
	if err := c.Do(ctx, "GET", p, nil, &got); err != nil {
		diags.AddError("Read organization setting failed", fmt.Sprintf("%s: %s", p, err.Error()))
		return types.ObjectNull(enabledAttrTypes), diags
	}
	obj, d := types.ObjectValue(enabledAttrTypes, map[string]attr.Value{"enabled": types.BoolValue(got.Enabled)})
	diags.Append(d...)
	return obj, diags
}

func patchEnabledBlock(ctx context.Context, c *client.Client, p string, obj types.Object) diag.Diagnostics {
	var diags diag.Diagnostics
	var m enabledModel
	diags.Append(obj.As(ctx, &m, basetypes.ObjectAsOptions{})...)
	if diags.HasError() {
		return diags
	}
	if err := c.Do(ctx, "PATCH", p, enabledPatch{Enabled: m.Enabled.ValueBool()}, nil); err != nil {
		diags.AddError("Update organization setting failed", fmt.Sprintf("%s: %s", p, err.Error()))
	}
	return diags
}

func isConfigured(o types.Object) bool {
	return !o.IsNull() && !o.IsUnknown()
}
