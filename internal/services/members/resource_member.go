// Package members implements the sigma_member resource and its data sources.
//
// Note: DELETE /v2/members/{memberId} *deactivates* the member rather than
// hard-deleting. Sigma retains the user for audit; recreating the same email
// returns a fresh memberId. The resource documents this in its description.
package members

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*memberResource)(nil)
	_ resource.ResourceWithConfigure   = (*memberResource)(nil)
	_ resource.ResourceWithImportState = (*memberResource)(nil)
)

func NewResource() resource.Resource { return &memberResource{} }

type memberResource struct {
	c *client.Client
}

// memberModel is the Terraform-side state for sigma_member.
type memberModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Email          types.String `tfsdk:"email"`
	FirstName      types.String `tfsdk:"first_name"`
	LastName       types.String `tfsdk:"last_name"`
	MemberType     types.String `tfsdk:"member_type"`
	UserKind       types.String `tfsdk:"user_kind"`
	Language       types.String `tfsdk:"language"`
	FormatRegion   types.String `tfsdk:"format_region"`
	SendInvite     types.Bool   `tfsdk:"send_invite"`
	HomeFolderID   types.String `tfsdk:"home_folder_id"`
	ProfileImgURL  types.String `tfsdk:"profile_img_url"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
	CreatedBy      types.String `tfsdk:"created_by"`
	UpdatedBy      types.String `tfsdk:"updated_by"`
}

func (r *memberResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member"
}

func (r *memberResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma member (user). Destroying this resource deactivates the user (Sigma retains audit history); it is not a hard delete.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Sigma member identifier (`memberId`). Opaque string; not necessarily a UUID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"organization_id": schema.StringAttribute{
				Computed:    true,
				Description: "Sigma organization UUID this member belongs to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "Member's email address. Must be unique within the org.",
			},
			"first_name": schema.StringAttribute{
				Required:    true,
				Description: "First name.",
			},
			"last_name": schema.StringAttribute{
				Required:    true,
				Description: "Last name.",
			},
			"member_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Account type's canonical license name as Sigma stores it (e.g. 'admin', 'build', 'view', or a custom account type name). " +
					"Note: Sigma normalizes some UI labels — the \"Creator\" account type is stored as 'build' — so passing the UI label (e.g. 'creator') causes an \"inconsistent result after apply\" error. Use the stored value. " +
					"If omitted, Sigma applies the organization's Invitation default account type (Admin > Account types); the assigned value is then read back into state.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_kind": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "One of `internal`, `guest`, `embed`. Defaults to `internal` server-side.",
			},
			"language": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Preferred display language, e.g. `EN`, `DE_DE`, `JA_JA` (Beta). See the Sigma API reference for the full list.",
			},
			"format_region": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Region whose number, date and currency conventions apply, e.g. `US`, `GB`, `JP` (Beta).",
			},
			"send_invite": schema.BoolAttribute{
				Optional: true,
				Description: "Whether Sigma should email an invitation to the new member. " +
					"Defaults to `true` (Sigma's API default). Set to `false` for silent " +
					"provisioning (e.g. when an external IdP handles user onboarding). " +
					"**Only applied at create time** — changes to this attribute on an " +
					"existing member are ignored. To suppress the diff after flipping it " +
					"post-creation, use `lifecycle { ignore_changes = [send_invite] }`.",
			},
			"home_folder_id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the user's My Documents folder.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"profile_img_url": schema.StringAttribute{
				Computed:    true,
				Description: "URL of the user's profile image, or empty.",
			},
			"created_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC3339 timestamp of member creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": schema.StringAttribute{
				Computed:    true,
				Description: "RFC3339 timestamp of last update.",
			},
			"created_by": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the member who created this member.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_by": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the member who last updated this member.",
			},
		},
	}
}

func (r *memberResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T. This is a provider bug.", req.ProviderData),
		)
		return
	}
	r.c = c
}

func (r *memberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer client.InvalidateList(r.c, membersListKey)
	var plan memberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := createMemberReq{
		Email:     plan.Email.ValueString(),
		FirstName: plan.FirstName.ValueString(),
		LastName:  plan.LastName.ValueString(),
	}
	if !plan.MemberType.IsNull() && !plan.MemberType.IsUnknown() {
		mt := plan.MemberType.ValueString()
		body.MemberType = &mt
	}
	if !plan.UserKind.IsNull() && !plan.UserKind.IsUnknown() {
		uk := plan.UserKind.ValueString()
		body.UserKind = &uk
	}
	if !plan.Language.IsNull() && !plan.Language.IsUnknown() {
		v := plan.Language.ValueString()
		body.Language = &v
	}
	if !plan.FormatRegion.IsNull() && !plan.FormatRegion.IsUnknown() {
		v := plan.FormatRegion.ValueString()
		body.FormatRegion = &v
	}

	url := "/v2/members"
	if !plan.SendInvite.IsNull() && !plan.SendInvite.IsUnknown() && !plan.SendInvite.ValueBool() {
		// Sigma's API default is sendInvite=true. Only attach the query param when
		// the user explicitly opts out, so logs stay quiet for the common case.
		url += "?sendInvite=false"
	}

	var created member
	if err := r.c.Do(ctx, "POST", url, body, &created); err != nil {
		if client.IsConflict(err) {
			// Sigma's POST /v2/members occasionally creates the member
			// server-side but still returns 409 with the new memberId in the
			// error message. Treat that case as a successful create — look up
			// the member by ID, verify the email matches what we sent, and
			// adopt it. If the email doesn't match, this is a genuine conflict
			// (caller intends to manage a pre-existing member) — fall back to
			// the import-required error.
			if memberID := extractMemberIDFromConflict(err); memberID != "" {
				var existing member
				if getErr := r.c.Do(ctx, "GET", "/v2/members/"+client.EscapeID(memberID), nil, &existing); getErr == nil && existing.Email == body.Email {
					applyToState(&existing, &plan)
					resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
					return
				}
			}
			resp.Diagnostics.AddError(
				"Member already exists",
				fmt.Sprintf("A Sigma member with email %q already exists. Run `terraform import sigma_member.<name> <memberId>` to adopt it. Underlying error: %s",
					body.Email, err),
			)
			return
		}
		resp.Diagnostics.AddError("Create member failed", err.Error())
		return
	}

	applyToState(&created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// extractMemberIDFromConflict pulls the memberId out of a Sigma 409 error
// message of the form "... (member-id=XXX) ...". Returns "" if not found.
func extractMemberIDFromConflict(err error) string {
	if err == nil {
		return ""
	}
	const marker = "member-id="
	s := err.Error()
	i := strings.Index(s, marker)
	if i < 0 {
		return ""
	}
	rest := s[i+len(marker):]
	end := strings.IndexAny(rest, ") \t\n,")
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func (r *memberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state memberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var got member
	err := r.c.Do(ctx, "GET", "/v2/members/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read member failed", err.Error())
		return
	}

	applyToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *memberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state memberModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := patchMemberReq{}
	if plan.Email.ValueString() != state.Email.ValueString() {
		v := plan.Email.ValueString()
		body.Email = &v
	}
	if plan.FirstName.ValueString() != state.FirstName.ValueString() {
		v := plan.FirstName.ValueString()
		body.FirstName = &v
	}
	if plan.LastName.ValueString() != state.LastName.ValueString() {
		v := plan.LastName.ValueString()
		body.LastName = &v
	}
	if plan.MemberType.ValueString() != state.MemberType.ValueString() {
		v := plan.MemberType.ValueString()
		body.MemberType = &v
	}
	if plan.UserKind.ValueString() != state.UserKind.ValueString() && !plan.UserKind.IsNull() && !plan.UserKind.IsUnknown() {
		v := plan.UserKind.ValueString()
		body.UserKind = &v
	}
	if !plan.Language.IsNull() && !plan.Language.IsUnknown() && plan.Language.ValueString() != state.Language.ValueString() {
		v := plan.Language.ValueString()
		body.Language = &v
	}
	if !plan.FormatRegion.IsNull() && !plan.FormatRegion.IsUnknown() && plan.FormatRegion.ValueString() != state.FormatRegion.ValueString() {
		v := plan.FormatRegion.ValueString()
		body.FormatRegion = &v
	}

	var got member
	if body != (patchMemberReq{}) {
		if err := r.c.Do(ctx, "PATCH", "/v2/members/"+client.EscapeID(state.ID.ValueString()), body, &got); err != nil {
			resp.Diagnostics.AddError("Update member failed", err.Error())
			return
		}
	}
	// PATCH returns the member; re-read only if it didn't (or nothing was sent).
	if got.MemberID == "" {
		if err := r.c.Do(ctx, "GET", "/v2/members/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
			resp.Diagnostics.AddError("Re-read after update failed", err.Error())
			return
		}
	}
	applyToState(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *memberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state memberModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, membersListKey)
	err := r.c.Do(ctx, "DELETE", "/v2/members/"+client.EscapeID(state.ID.ValueString()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete (deactivate) member failed", err.Error())
		return
	}
}

func (r *memberResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// applyToState writes fields from an API response into the TF state model.
func applyToState(m *member, dst *memberModel) {
	dst.ID = types.StringValue(m.MemberID)
	dst.OrganizationID = types.StringValue(m.OrganizationID)
	dst.Email = types.StringValue(m.Email)
	dst.FirstName = types.StringValue(m.FirstName)
	dst.LastName = types.StringValue(m.LastName)
	dst.MemberType = types.StringValue(m.MemberType)
	dst.UserKind = types.StringValue(m.UserKind)
	dst.HomeFolderID = types.StringValue(m.HomeFolderID)
	dst.Language = types.StringPointerValue(m.Language)
	dst.FormatRegion = types.StringPointerValue(m.FormatRegion)
	if m.ProfileImgURL != nil {
		dst.ProfileImgURL = types.StringValue(*m.ProfileImgURL)
	} else {
		dst.ProfileImgURL = types.StringValue("")
	}
	dst.CreatedAt = types.StringValue(m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.UpdatedAt = types.StringValue(m.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"))
	dst.CreatedBy = types.StringValue(m.CreatedBy)
	dst.UpdatedBy = types.StringValue(m.UpdatedBy)
}
