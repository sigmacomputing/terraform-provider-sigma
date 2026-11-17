// Package smtp implements sigma_smtp_settings.
package smtp

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

// singletonID: one SMTP config per org.
const singletonID = "smtp"

const smtpPath = "/v2/organizations/settings/smtp"

var (
	_ resource.Resource                = (*smtpResource)(nil)
	_ resource.ResourceWithConfigure   = (*smtpResource)(nil)
	_ resource.ResourceWithImportState = (*smtpResource)(nil)
)

func NewResource() resource.Resource { return &smtpResource{} }

type smtpResource struct{ c *client.Client }

type smtpModel struct {
	ID                 types.String `tfsdk:"id"`
	Host               types.String `tfsdk:"host"`
	Port               types.String `tfsdk:"port"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	SenderEmail        types.String `tfsdk:"sender_email"`
	SenderName         types.String `tfsdk:"sender_name"`
	GlobalBccAddresses types.List   `tfsdk:"global_bcc_addresses"`
	ReplyToAddress     types.String `tfsdk:"reply_to_address"`
}

type smtpPatchReq struct {
	Host               string    `json:"host"`
	Port               string    `json:"port"`
	Username           string    `json:"username"`
	SenderEmail        string    `json:"senderEmail"`
	Password           string    `json:"password"`
	SenderName         *string   `json:"senderName,omitempty"`
	GlobalBccAddresses *[]string `json:"globalBccAddresses,omitempty"`
	ReplyToAddress     *string   `json:"replyToAddress,omitempty"`
}

type smtpResp struct {
	Host               string   `json:"host"`
	Port               string   `json:"port"`
	Username           string   `json:"username"`
	SenderEmail        string   `json:"senderEmail"`
	SenderName         string   `json:"senderName"`
	GlobalBccAddresses []string `json:"globalBccAddresses"`
	ReplyToAddress     *string  `json:"replyToAddress"`
}

func (r *smtpResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_smtp_settings"
}

func (r *smtpResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Singleton resource managing the organization's custom SMTP server for outbound email. Destroying it deletes the SMTP settings. The password is write-only: Sigma never returns it, so changes made outside Terraform to the password are not detected.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"host":     schema.StringAttribute{Required: true, Description: "SMTP server hostname."},
			"port":     schema.StringAttribute{Required: true, Description: "SMTP server port, as a string."},
			"username": schema.StringAttribute{Required: true, Description: "SMTP username."},
			"password": schema.StringAttribute{Required: true, Sensitive: true, Description: "SMTP password. Never read back from Sigma."},
			"sender_email": schema.StringAttribute{Required: true,
				Description: "Address emails are sent from."},
			"sender_name": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Display name emails are sent from."},
			"global_bcc_addresses": schema.ListAttribute{Optional: true, Computed: true, ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "Addresses BCC'd on every outbound email."},
			"reply_to_address": schema.StringAttribute{Optional: true, Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Reply-to address for outbound email."},
		},
	}
}

func (r *smtpResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *smtpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan smtpModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *smtpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan smtpModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// apply PATCHes the plan and writes the server's view back into it (password stays as planned).
func (r *smtpResource) apply(ctx context.Context, plan *smtpModel, diags *diag.Diagnostics) {
	body := smtpPatchReq{
		Host:        plan.Host.ValueString(),
		Port:        plan.Port.ValueString(),
		Username:    plan.Username.ValueString(),
		SenderEmail: plan.SenderEmail.ValueString(),
		Password:    plan.Password.ValueString(),
	}
	if !plan.SenderName.IsNull() && !plan.SenderName.IsUnknown() {
		v := plan.SenderName.ValueString()
		body.SenderName = &v
	}
	if !plan.ReplyToAddress.IsNull() && !plan.ReplyToAddress.IsUnknown() {
		v := plan.ReplyToAddress.ValueString()
		body.ReplyToAddress = &v
	}
	if !plan.GlobalBccAddresses.IsNull() && !plan.GlobalBccAddresses.IsUnknown() {
		var ids []string
		for _, e := range plan.GlobalBccAddresses.Elements() {
			ids = append(ids, e.(types.String).ValueString())
		}
		if ids == nil {
			ids = []string{}
		}
		body.GlobalBccAddresses = &ids
	}
	var got smtpResp
	if err := r.c.Do(ctx, "PATCH", smtpPath, body, &got); err != nil {
		diags.AddError("Update SMTP settings failed", err.Error())
		return
	}
	if err := fill(ctx, plan, got); err != nil {
		diags.AddError("Decode SMTP settings failed", err.Error())
	}
}

func (r *smtpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state smtpModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got smtpResp
	err := r.c.Do(ctx, "GET", smtpPath, nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read SMTP settings failed", err.Error())
		return
	}
	if err := fill(ctx, &state, got); err != nil {
		resp.Diagnostics.AddError("Decode SMTP settings failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *smtpResource) Delete(ctx context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	if err := r.c.Do(ctx, "DELETE", smtpPath, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete SMTP settings failed", err.Error())
	}
}

func (r *smtpResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Password can't be imported; set it in config and the next apply writes it.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), singletonID)...)
}

// fill copies the API view onto the model, leaving the write-only password untouched.
func fill(_ context.Context, m *smtpModel, got smtpResp) error {
	m.ID = types.StringValue(singletonID)
	m.Host = types.StringValue(got.Host)
	m.Port = types.StringValue(got.Port)
	m.Username = types.StringValue(got.Username)
	m.SenderEmail = types.StringValue(got.SenderEmail)
	m.SenderName = types.StringValue(got.SenderName)
	if got.ReplyToAddress != nil {
		m.ReplyToAddress = types.StringValue(*got.ReplyToAddress)
	} else {
		m.ReplyToAddress = types.StringNull()
	}
	elems := make([]types.String, 0, len(got.GlobalBccAddresses))
	for _, a := range got.GlobalBccAddresses {
		elems = append(elems, types.StringValue(a))
	}
	l, diags := types.ListValueFrom(context.Background(), types.StringType, elems)
	if diags.HasError() {
		return fmt.Errorf("%v", diags)
	}
	m.GlobalBccAddresses = l
	return nil
}
