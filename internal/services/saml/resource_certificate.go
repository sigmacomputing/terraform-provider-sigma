// Package saml implements sigma_saml_service_provider_certificate (Beta API).
package saml

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*certificateResource)(nil)
	_ resource.ResourceWithConfigure   = (*certificateResource)(nil)
	_ resource.ResourceWithImportState = (*certificateResource)(nil)
)

func NewCertificateResource() resource.Resource { return &certificateResource{} }

type certificateResource struct{ c *client.Client }

type certificateModel struct {
	ID                   types.String `tfsdk:"id"`
	ServiceProviderID    types.String `tfsdk:"saml_service_provider_id"`
	Purpose              types.String `tfsdk:"purpose"`
	ValidForDays         types.Int64  `tfsdk:"valid_for_days"`
	Active               types.Bool   `tfsdk:"active"`
	PublicKeyCertificate types.String `tfsdk:"public_key_certificate"`
	NotBefore            types.String `tfsdk:"not_before"`
	NotAfter             types.String `tfsdk:"not_after"`
}

type createCertReq struct {
	Purpose      string `json:"purpose"`
	ValidForDays int64  `json:"validForDays"`
}

type createCertResp struct {
	CertificateID string `json:"samlServiceProviderCertificateId"`
}

type certResp struct {
	CertificateID        string `json:"samlServiceProviderCertificateId"`
	Purpose              string `json:"purpose"`
	PublicKeyCertificate string `json:"publicKeyCertificate"`
	IsActive             bool   `json:"isActive"`
	NotBefore            string `json:"notBefore"`
	NotAfter             string `json:"notAfter"`
}

func certsPath(spID string) string {
	return "/v2/saml/service-providers/" + client.EscapeID(spID) + "/certificates"
}

func certPath(spID, certID string) string { return certsPath(spID) + "/" + client.EscapeID(certID) }

func (r *certificateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_saml_service_provider_certificate"
}

func (r *certificateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Certificate for a SAML service provider (Beta API; behaviour may change). Sigma generates the key pair; `active` activates or deactivates it. Composite ID `<saml_service_provider_id>:<certificate_id>`. Look up service provider IDs with data.sigma_saml_service_providers.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"saml_service_provider_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"purpose": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "`signing` or `encryption`."},
			"valid_for_days": schema.Int64Attribute{Required: true,
				Description: "Certificate lifetime in days, used only at creation. Not returned by Sigma, so changing it later has no effect; use `terraform apply -replace` to reissue."},
			"active": schema.BoolAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether the certificate is active. Defaults to the server state (new certificates are created inactive)."},
			"public_key_certificate": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "PEM public certificate to register with the identity provider."},
			"not_before": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"not_after": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
		},
	}
}

func (r *certificateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *certificateResource) setActive(ctx context.Context, spID, certID string, active bool) error {
	action := "deactivate"
	if active {
		action = "activate"
	}
	return r.c.Do(ctx, "POST", certPath(spID, certID)+"/"+action, struct{}{}, nil)
}

func (r *certificateResource) refresh(ctx context.Context, m *certificateModel, spID, certID string) error {
	var got certResp
	if err := r.c.Do(ctx, "GET", certPath(spID, certID), nil, &got); err != nil {
		return err
	}
	m.ID = types.StringValue(spID + ":" + certID)
	m.Purpose = types.StringValue(got.Purpose)
	m.Active = types.BoolValue(got.IsActive)
	m.PublicKeyCertificate = types.StringValue(got.PublicKeyCertificate)
	m.NotBefore = types.StringValue(got.NotBefore)
	m.NotAfter = types.StringValue(got.NotAfter)
	return nil
}

func (r *certificateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan certificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	spID := plan.ServiceProviderID.ValueString()
	var created createCertResp
	body := createCertReq{Purpose: plan.Purpose.ValueString(), ValidForDays: plan.ValidForDays.ValueInt64()}
	if err := r.c.Do(ctx, "POST", certsPath(spID), body, &created); err != nil {
		resp.Diagnostics.AddError("Create SAML certificate failed", err.Error())
		return
	}
	// Persist the ID first so a failed activate doesn't orphan the certificate.
	plan.ID = types.StringValue(spID + ":" + created.CertificateID)
	wantActive := !plan.Active.IsNull() && !plan.Active.IsUnknown() && plan.Active.ValueBool()
	if wantActive {
		if err := r.setActive(ctx, spID, created.CertificateID, true); err != nil {
			resp.Diagnostics.AddError("Activate SAML certificate failed", err.Error())
			plan.Active = types.BoolValue(false)
			plan.PublicKeyCertificate, plan.NotBefore, plan.NotAfter = types.StringNull(), types.StringNull(), types.StringNull()
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			return
		}
	}
	if err := r.refresh(ctx, &plan, spID, created.CertificateID); err != nil {
		resp.Diagnostics.AddError("Read SAML certificate failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *certificateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state certificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	spID, certID, ok := splitID(state.ID.ValueString())
	if !ok {
		resp.Diagnostics.AddError("Invalid state ID", state.ID.ValueString())
		return
	}
	state.ServiceProviderID = types.StringValue(spID)
	if err := r.refresh(ctx, &state, spID, certID); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read SAML certificate failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *certificateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state certificateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	spID, certID, ok := splitID(state.ID.ValueString())
	if !ok {
		resp.Diagnostics.AddError("Invalid state ID", state.ID.ValueString())
		return
	}
	if plan.Active.ValueBool() != state.Active.ValueBool() {
		if err := r.setActive(ctx, spID, certID, plan.Active.ValueBool()); err != nil {
			resp.Diagnostics.AddError("Update SAML certificate state failed", err.Error())
			return
		}
	}
	if err := r.refresh(ctx, &plan, spID, certID); err != nil {
		resp.Diagnostics.AddError("Read SAML certificate failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *certificateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state certificateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	spID, certID, ok := splitID(state.ID.ValueString())
	if !ok {
		resp.Diagnostics.AddError("Invalid state ID", state.ID.ValueString())
		return
	}
	// Best effort: an active certificate may need deactivating before removal.
	if state.Active.ValueBool() {
		_ = r.setActive(ctx, spID, certID, false)
	}
	if err := r.c.Do(ctx, "DELETE", certPath(spID, certID), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Remove SAML certificate failed", err.Error())
	}
}

func (r *certificateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	spID, _, ok := splitID(req.ID)
	if !ok {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<saml_service_provider_id>:<certificate_id>`, got "+req.ID)
		return
	}
	// valid_for_days isn't returned; it's creation-only, so any config value is accepted after import.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("saml_service_provider_id"), spID)...)
}

func splitID(id string) (spID, certID string, ok bool) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}
