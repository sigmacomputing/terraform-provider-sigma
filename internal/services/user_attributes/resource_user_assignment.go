package user_attributes

import (
	"context"
	"fmt"
	"net/url"
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
	_ resource.Resource                = (*userAssignmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*userAssignmentResource)(nil)
	_ resource.ResourceWithImportState = (*userAssignmentResource)(nil)
)

func NewUserAssignmentResource() resource.Resource { return &userAssignmentResource{} }

type userAssignmentResource struct{ c *client.Client }

type userAssignmentModel struct {
	ID              types.String `tfsdk:"id"`
	UserAttributeID types.String `tfsdk:"user_attribute_id"`
	UserID          types.String `tfsdk:"user_id"`
	Value           types.String `tfsdk:"value"`
}

func (r *userAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute_user_assignment"
}

func (r *userAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a user attribute value to a single user. Composite ID `<user_attribute_id>:<user_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_attribute_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"user_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"value": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "String value assigned to this user. Marked Sensitive because UA values commonly carry tenant IDs / PII-adjacent identifiers used for row-level security.",
			},
		},
	}
}

func (r *userAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *userAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := plan.UserAttributeID.ValueString()
	body := assignmentsBody[userAssignment]{
		Assignments: []userAssignment{{UserID: plan.UserID.ValueString(), Value: defaultValue{Val: plan.Value.ValueString(), Type: "string"}}},
	}
	defer client.InvalidateList(r.c, uaUsersKey(uaID))
	if err := r.c.Do(ctx, "POST", "/v2/user-attributes/"+client.EscapeID(uaID)+"/users", body, nil); err != nil {
		resp.Diagnostics.AddError("Assign user attribute to user failed", err.Error())
		return
	}
	plan.ID = types.StringValue(uaID + ":" + plan.UserID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := state.UserAttributeID.ValueString()
	userID := state.UserID.ValueString()
	all, err := client.CachedList(r.c, uaUsersKey(uaID), func() ([]userAssignment, error) {
		return client.ListAllPaged[userAssignment](ctx, r.c, "/v2/user-attributes/"+client.EscapeID(uaID)+"/users", url.Values{})
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List user assignments failed", err.Error())
		return
	}
	for _, a := range all {
		if a.UserID == userID {
			state.Value = types.StringValue(a.Value.Val)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *userAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Re-POST overwrites the value (Sigma's POST is upsert for individual assignments).
	var plan userAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := plan.UserAttributeID.ValueString()
	body := assignmentsBody[userAssignment]{
		Assignments: []userAssignment{{UserID: plan.UserID.ValueString(), Value: defaultValue{Val: plan.Value.ValueString(), Type: "string"}}},
	}
	defer client.InvalidateList(r.c, uaUsersKey(uaID))
	if err := r.c.Do(ctx, "POST", "/v2/user-attributes/"+client.EscapeID(uaID)+"/users", body, nil); err != nil {
		resp.Diagnostics.AddError("Update user assignment failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := deleteUserAssignmentReq{}
	body.Delete = append(body.Delete, struct {
		UserID string `json:"userId"`
	}{UserID: state.UserID.ValueString()})
	defer client.InvalidateList(r.c, uaUsersKey(state.UserAttributeID.ValueString()))
	if err := r.c.Do(ctx, "PATCH", "/v2/user-attributes/"+client.EscapeID(state.UserAttributeID.ValueString())+"/users", body, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete user assignment failed", err.Error())
	}
}

// uaUsersKey is the per-run CachedList key for a user attribute's user assignments.
func uaUsersKey(uaID string) string { return "ua-users:" + uaID }

func (r *userAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<user_attribute_id>:<user_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_attribute_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), parts[1])...)
}
