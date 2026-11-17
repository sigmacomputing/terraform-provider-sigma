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
	_ resource.Resource                = (*teamAssignmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*teamAssignmentResource)(nil)
	_ resource.ResourceWithImportState = (*teamAssignmentResource)(nil)
)

func NewTeamAssignmentResource() resource.Resource { return &teamAssignmentResource{} }

type teamAssignmentResource struct{ c *client.Client }

type teamAssignmentModel struct {
	ID              types.String `tfsdk:"id"`
	UserAttributeID types.String `tfsdk:"user_attribute_id"`
	TeamID          types.String `tfsdk:"team_id"`
	Value           types.String `tfsdk:"value"`
}

func (r *teamAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_attribute_team_assignment"
}

func (r *teamAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Assigns a user attribute value to a team. Composite ID `<user_attribute_id>:<team_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"user_attribute_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"team_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"value": schema.StringAttribute{Required: true, Sensitive: true, Description: "String value assigned to this team. Marked Sensitive because UA values commonly carry tenant IDs / PII-adjacent identifiers used for row-level security."},
		},
	}
}

func (r *teamAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *teamAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := plan.UserAttributeID.ValueString()
	body := assignmentsBody[teamAssignment]{
		Assignments: []teamAssignment{{TeamID: plan.TeamID.ValueString(), Value: defaultValue{Val: plan.Value.ValueString(), Type: "string"}}},
	}
	defer client.InvalidateList(r.c, uaTeamsKey(uaID))
	if err := r.c.Do(ctx, "POST", "/v2/user-attributes/"+client.EscapeID(uaID)+"/teams", body, nil); err != nil {
		resp.Diagnostics.AddError("Assign user attribute to team failed", err.Error())
		return
	}
	plan.ID = types.StringValue(uaID + ":" + plan.TeamID.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := state.UserAttributeID.ValueString()
	teamID := state.TeamID.ValueString()
	all, err := client.CachedList(r.c, uaTeamsKey(uaID), func() ([]teamAssignment, error) {
		return client.ListAllPaged[teamAssignment](ctx, r.c, "/v2/user-attributes/"+client.EscapeID(uaID)+"/teams", url.Values{})
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List team assignments failed", err.Error())
		return
	}
	for _, a := range all {
		if a.TeamID == teamID {
			state.Value = types.StringValue(a.Value.Val)
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *teamAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan teamAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	uaID := plan.UserAttributeID.ValueString()
	body := assignmentsBody[teamAssignment]{
		Assignments: []teamAssignment{{TeamID: plan.TeamID.ValueString(), Value: defaultValue{Val: plan.Value.ValueString(), Type: "string"}}},
	}
	defer client.InvalidateList(r.c, uaTeamsKey(uaID))
	if err := r.c.Do(ctx, "POST", "/v2/user-attributes/"+client.EscapeID(uaID)+"/teams", body, nil); err != nil {
		resp.Diagnostics.AddError("Update team assignment failed", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := deleteTeamAssignmentReq{}
	body.Delete = append(body.Delete, struct {
		TeamID string `json:"teamId"`
	}{TeamID: state.TeamID.ValueString()})
	defer client.InvalidateList(r.c, uaTeamsKey(state.UserAttributeID.ValueString()))
	if err := r.c.Do(ctx, "PATCH", "/v2/user-attributes/"+client.EscapeID(state.UserAttributeID.ValueString())+"/teams", body, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete team assignment failed", err.Error())
	}
}

func (r *teamAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<user_attribute_id>:<team_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_attribute_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), parts[1])...)
}

// uaTeamsKey is the per-run CachedList key for a user attribute's team assignments.
func uaTeamsKey(uaID string) string { return "ua-teams:" + uaID }
