// Package account_types implements sigma_account_type. Sigma's API has no
// rename endpoint, so a name change is emulated by migrating members to a new
// type (rename.go); a description-only change replaces. Permissions are updated
// in place via PATCH /v2/accountTypes/{id}/permissions.
package account_types

import (
	"context"
	"fmt"
	"net/url"

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
	_ resource.Resource                = (*accountTypeResource)(nil)
	_ resource.ResourceWithConfigure   = (*accountTypeResource)(nil)
	_ resource.ResourceWithImportState = (*accountTypeResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*accountTypeResource)(nil)
)

func NewResource() resource.Resource { return &accountTypeResource{} }

type accountTypeResource struct{ c *client.Client }

type accountTypeModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	Permissions             types.Set    `tfsdk:"permissions"`
	IsCustom                types.Bool   `tfsdk:"is_custom"`
	ReassignToAccountTypeID types.String `tfsdk:"reassign_to_account_type_id"`
	MigrateOnRename         types.Bool   `tfsdk:"migrate_on_rename"`
}

// builtInViewAccountTypeID is Sigma's stable, well-known account type ID for
// "view". DELETE /v2/accountTypes/{id} requires reassignToAccountTypeId even
// when no members are using the type — undocumented in the OAS but enforced
// by the API. We default to view so destroys don't blow up; users can override
// via the `reassign_to_account_type_id` attribute.
const builtInViewAccountTypeID = "20140502-5fca-4000-8000-000000000004"

type accountType struct {
	AccountTypeID   string `json:"accountTypeId"`
	AccountTypeName string `json:"accountTypeName"`
	Description     string `json:"description"`
	IsCustom        bool   `json:"isCustom"`
	LicenseType     string `json:"licenseType"`
}

// patchPermissionsReq is the PATCH /v2/accountTypes/{id}/permissions body.
type patchPermissionsReq struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

type createAccountTypeReq struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}

func (r *accountTypeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_account_type"
}

func (r *accountTypeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A custom Sigma account type. Built-in account types (admin, creator, explorer, viewer) cannot be managed via this resource — only `is_custom = true` types are mutable. Changing `permissions` updates the account type in place; changing `name` or `description` replaces it (Sigma has no update endpoint for them), which reassigns the type's members to `reassign_to_account_type_id`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Account type UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Account type name. Sigma has no rename endpoint, so a change is applied by creating the new type, moving its members, and deleting the old one (see `migrate_on_rename`); the account type `id` changes.",
			},
			"description": schema.StringAttribute{
				Required:    true,
				Description: "Human-readable description. Changing it replaces the account type (Sigma has no update endpoint); when `name` also changes it is applied by the rename migration instead.",
			},
			"permissions": schema.SetAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Permissions enabled for this account type. Use the names from the `sigma_account_type_permissions` data source. Updated in place.",
			},
			"is_custom": schema.BoolAttribute{
				Computed:      true,
				Description:   "Always `true` for resources managed here.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"migrate_on_rename": schema.BoolAttribute{
				Optional: true,
				Description: "Defaults to `true`. When `name` changes, move the type's members to a new type with the new name before deleting the old one, so nobody is reassigned to the delete fallback. " +
					"Set to `false` to get a plain replace (the old type is deleted and its members go to `reassign_to_account_type_id`).",
			},
			"reassign_to_account_type_id": schema.StringAttribute{
				Optional: true,
				Description: "Account type UUID that members of this type are reassigned to on destroy. " +
					"Sigma's DELETE endpoint requires this even when no members are assigned. " +
					"Defaults to the built-in `view` account type (`" + builtInViewAccountTypeID + "`).",
			},
		},
	}
}

func (r *accountTypeResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *accountTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accountTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var perms []string
	resp.Diagnostics.Append(plan.Permissions.ElementsAs(ctx, &perms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createAccountTypeReq{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Permissions: perms,
	}
	defer client.InvalidateList(r.c, accountTypesKey)
	var got accountType
	if err := r.c.Do(ctx, "POST", "/v2/accountTypes", body, &got); err != nil {
		resp.Diagnostics.AddError("Create account type failed", err.Error())
		return
	}
	plan.ID = types.StringValue(got.AccountTypeID)
	plan.IsCustom = types.BoolValue(got.IsCustom)
	// Server may normalize name/description; trust the response.
	plan.Name = types.StringValue(got.AccountTypeName)
	plan.Description = types.StringValue(got.Description)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *accountTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state accountTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// No GET-by-id endpoint; list all and filter. Account types are few (<100),
	// but memoize the list per run so refreshing N account types is one list, not N.
	all, err := listAccountTypes(ctx, r.c)
	if err != nil {
		resp.Diagnostics.AddError("List account types failed", err.Error())
		return
	}
	var found *accountType
	for i := range all {
		if all[i].AccountTypeID == state.ID.ValueString() {
			found = &all[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.Name = types.StringValue(found.AccountTypeName)
	state.Description = types.StringValue(found.Description)
	state.IsCustom = types.BoolValue(found.IsCustom)

	// Permissions live on a separate endpoint that returns a bare array.
	type permEntry struct {
		Permission string `json:"permission"`
	}
	perms, err := client.ListAllArray[permEntry](ctx, r.c, "/v2/accountTypes/"+client.EscapeID(state.ID.ValueString())+"/permissions")
	if err != nil {
		resp.Diagnostics.AddError("List permissions for account type failed", err.Error())
		return
	}
	// The API can repeat a permission name; a set rejects duplicates.
	seen := make(map[string]struct{}, len(perms))
	names := make([]string, 0, len(perms))
	for _, p := range perms {
		if _, dup := seen[p.Permission]; dup {
			continue
		}
		seen[p.Permission] = struct{}{}
		names = append(names, p.Permission)
	}
	permsSet, diags := types.SetValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Permissions = permsSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ModifyPlan decides how a name/description change is applied: a rename migrates
// in place (new id), a description-only change or migrate_on_rename = false replaces.
func (r *accountTypeResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state accountTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	nameChanged := plan.Name.ValueString() != state.Name.ValueString()
	descChanged := plan.Description.ValueString() != state.Description.ValueString()
	if !nameChanged && !descChanged {
		return
	}
	migrate := plan.MigrateOnRename.IsNull() || plan.MigrateOnRename.ValueBool()
	if migrate && nameChanged {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
		resp.Diagnostics.AddAttributeWarning(path.Root("name"),
			"Renaming creates a new account type",
			fmt.Sprintf("Sigma cannot rename an account type, so %q is applied by creating %q, moving its members across, and deleting the old type. "+
				"The account type gets a new id. Members move directly and are not reassigned to the delete fallback, "+
				"but settings that reference the old type by id or name (for example SSO or SCIM role mappings) will not follow it.",
				state.Name.ValueString(), plan.Name.ValueString()))
		return
	}
	if nameChanged {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("name"))
	}
	if descChanged {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("description"))
	}
}

func (r *accountTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accountTypeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var want, have []string
	resp.Diagnostics.Append(plan.Permissions.ElementsAs(ctx, &want, false)...)
	if plan.Name.ValueString() != state.Name.ValueString() {
		// ModifyPlan only lets a name change reach Update when migrating.
		reassign := builtInViewAccountTypeID
		if !plan.ReassignToAccountTypeID.IsNull() && !plan.ReassignToAccountTypeID.IsUnknown() && plan.ReassignToAccountTypeID.ValueString() != "" {
			reassign = plan.ReassignToAccountTypeID.ValueString()
		}
		defer client.InvalidateList(r.c, accountTypesKey)
		got, err := migrateAccountType(ctx, clientRenameAPI{r.c}, renameInput{
			OldID: state.ID.ValueString(), OldName: state.Name.ValueString(),
			NewName: plan.Name.ValueString(), NewDescription: plan.Description.ValueString(),
			Permissions: want, ReassignTo: reassign,
		})
		if err != nil {
			resp.Diagnostics.AddError("Rename account type failed", err.Error())
			return
		}
		plan.ID = types.StringValue(got.AccountTypeID)
		plan.IsCustom = types.BoolValue(got.IsCustom)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}
	resp.Diagnostics.Append(state.Permissions.ElementsAs(ctx, &have, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	add, remove := diffPermissions(have, want)
	if len(add) > 0 || len(remove) > 0 {
		body := patchPermissionsReq{Add: add, Remove: remove}
		url := "/v2/accountTypes/" + client.EscapeID(state.ID.ValueString()) + "/permissions"
		if err := r.c.Do(ctx, "PATCH", url, body, nil); err != nil {
			resp.Diagnostics.AddError("Update account type permissions failed", err.Error())
			return
		}
	}

	// name/description are RequiresReplace, so only permissions and reassign_to_account_type_id reach here.
	plan.ID = state.ID
	plan.IsCustom = state.IsCustom
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// diffPermissions returns the permissions to add to and remove from `have` to reach `want`.
func diffPermissions(have, want []string) (add, remove []string) {
	haveSet := make(map[string]struct{}, len(have))
	for _, p := range have {
		haveSet[p] = struct{}{}
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, p := range want {
		wantSet[p] = struct{}{}
		if _, ok := haveSet[p]; !ok {
			add = append(add, p)
		}
	}
	for _, p := range have {
		if _, ok := wantSet[p]; !ok {
			remove = append(remove, p)
		}
	}
	return add, remove
}

func (r *accountTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state accountTypeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	reassign := builtInViewAccountTypeID
	if !state.ReassignToAccountTypeID.IsNull() && !state.ReassignToAccountTypeID.IsUnknown() && state.ReassignToAccountTypeID.ValueString() != "" {
		reassign = state.ReassignToAccountTypeID.ValueString()
	}
	defer client.InvalidateList(r.c, accountTypesKey)
	delPath := "/v2/accountTypes/" + client.EscapeID(state.ID.ValueString()) + "?reassignToAccountTypeId=" + url.QueryEscape(reassign)
	if err := r.c.Do(ctx, "DELETE", delPath, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete account type failed", err.Error())
	}
}

// accountTypesKey is the per-run CachedList key for the org's account-type list.
const accountTypesKey = "account-types"

// listAccountTypes drains /v2/accountTypes (pageToken paging), memoized per run.
func listAccountTypes(ctx context.Context, c *client.Client) ([]accountType, error) {
	return client.CachedList(c, accountTypesKey, func() ([]accountType, error) {
		return client.ListAllPageToken[accountType](ctx, c, "/v2/accountTypes", client.PageSize(client.MaxPageSize))
	})
}

func (r *accountTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
