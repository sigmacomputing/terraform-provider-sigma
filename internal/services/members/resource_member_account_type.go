package members

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*memberAccountTypeResource)(nil)
	_ resource.ResourceWithConfigure   = (*memberAccountTypeResource)(nil)
	_ resource.ResourceWithImportState = (*memberAccountTypeResource)(nil)
)

// membersListKey caches the org member roster so N assignments cost one list, not N.
const membersListKey = "members"

// defaultResetTo is the account type a member falls back to when the assignment is destroyed.
const defaultResetTo = "Pro"

func NewMemberAccountTypeResource() resource.Resource { return &memberAccountTypeResource{} }

type memberAccountTypeResource struct {
	c *client.Client
}

type memberAccountTypeModel struct {
	ID          types.String `tfsdk:"id"`
	Email       types.String `tfsdk:"email"`
	AccountType types.String `tfsdk:"account_type"`
	ResetTo     types.String `tfsdk:"reset_to"`
}

func (r *memberAccountTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_member_account_type"
}

func (r *memberAccountTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns an account type to an EXISTING Sigma member without owning the member's lifecycle. " +
			"Use this when an external IdP (e.g. Okta) creates and deactivates users: this resource never creates a member and never deactivates one. " +
			"Create fails if the member does not exist. Destroying the resource demotes the member back to `reset_to`. " +
			"If the member is deactivated or removed out of band, the resource drops out of state instead of being recreated.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Sigma member identifier (`memberId`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email": schema.StringAttribute{
				Required:      true,
				Description:   "Email of the existing member. Matched case-insensitively. Changing it replaces the resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"account_type": schema.StringAttribute{
				Required: true,
				Description: "Account type name as Sigma stores it on the member (e.g. `admin`, `explorer`, `Pro`, or a custom account type name). " +
					"Sigma normalizes some UI labels, so use the stored value (see `sigma_members`).",
			},
			"reset_to": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString(defaultResetTo),
				Description: "Account type the member is moved to when this resource is destroyed. Defaults to `Pro`.",
			},
		},
	}
}

func (r *memberAccountTypeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// findMemberByEmail resolves an email to a member using the cached roster. Returns nil when absent.
func (r *memberAccountTypeResource) findMemberByEmail(ctx context.Context, email string) (*member, error) {
	all, err := client.CachedList(r.c, membersListKey, func() ([]member, error) {
		return client.ListAllPaged[member](ctx, r.c, "/v2.1/members", nil)
	})
	if err != nil {
		return nil, err
	}
	for i := range all {
		if strings.EqualFold(all[i].Email, email) {
			return &all[i], nil
		}
	}
	// The roster may predate a member created earlier in this apply.
	return lookupByEmail(ctx, r.c, email)
}

// lookupByEmail finds a member with the list endpoint's email filter, which
// matches substrings, so the exact match is checked here.
func lookupByEmail(ctx context.Context, c *client.Client, email string) (*member, error) {
	var found *member
	err := client.ForEachPaged[member](ctx, c, "/v2.1/members", url.Values{"email": {email}}, func(m member) (bool, error) {
		if strings.EqualFold(m.Email, email) {
			found = &m
			return true, nil
		}
		return false, nil
	})
	return found, err
}

func (r *memberAccountTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan memberAccountTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	listed, err := r.findMemberByEmail(ctx, plan.Email.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("List members failed", err.Error())
		return
	}
	if listed == nil {
		resp.Diagnostics.AddError(
			"Member not found",
			fmt.Sprintf("No active Sigma member with email %q. This resource only assigns an account type to an existing member; it never creates one (provision the user through your IdP first).", plan.Email.ValueString()),
		)
		return
	}

	var got member
	if err := r.c.Do(ctx, "GET", "/v2/members/"+client.EscapeID(listed.MemberID), nil, &got); err != nil {
		if client.IsNotFound(err) {
			resp.Diagnostics.AddError("Member not found", fmt.Sprintf("Member %q is listed but cannot be fetched (deactivated?): %s", plan.Email.ValueString(), err))
			return
		}
		resp.Diagnostics.AddError("Read member failed", err.Error())
		return
	}

	if got.MemberType != plan.AccountType.ValueString() {
		if err := r.patchMemberType(ctx, got.MemberID, plan.AccountType.ValueString()); err != nil {
			resp.Diagnostics.AddError("Assign account type failed", err.Error())
			return
		}
	}

	plan.ID = types.StringValue(got.MemberID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *memberAccountTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state memberAccountTypeModel
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
	if got.IsInactive != nil && *got.IsInactive {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Email = types.StringValue(got.Email)
	state.AccountType = types.StringValue(got.MemberType)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *memberAccountTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state memberAccountTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.AccountType.ValueString() != state.AccountType.ValueString() {
		if err := r.patchMemberType(ctx, state.ID.ValueString(), plan.AccountType.ValueString()); err != nil {
			resp.Diagnostics.AddError("Assign account type failed", err.Error())
			return
		}
	}

	plan.ID = state.ID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete demotes the member to reset_to; it never deactivates them.
func (r *memberAccountTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state memberAccountTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var got member
	err := r.c.Do(ctx, "GET", "/v2/members/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read member failed", err.Error())
		return
	}
	if got.IsInactive != nil && *got.IsInactive {
		return
	}

	resetTo := state.ResetTo.ValueString()
	if resetTo == "" {
		resetTo = defaultResetTo
	}
	if got.MemberType == resetTo {
		return
	}
	if err := r.patchMemberType(ctx, state.ID.ValueString(), resetTo); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Demote member failed", err.Error())
	}
}

// ImportState takes the member's email and resolves the memberId from the roster.
func (r *memberAccountTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	listed, err := r.findMemberByEmail(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("List members failed", err.Error())
		return
	}
	if listed == nil {
		resp.Diagnostics.AddError("Member not found", fmt.Sprintf("No active Sigma member with email %q.", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), listed.MemberID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), listed.Email)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("reset_to"), defaultResetTo)...)
}

func (r *memberAccountTypeResource) patchMemberType(ctx context.Context, memberID, accountType string) error {
	body := patchMemberReq{MemberType: &accountType}
	return r.c.Do(ctx, "PATCH", "/v2/members/"+client.EscapeID(memberID), body, nil)
}
