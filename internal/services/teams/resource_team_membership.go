package teams

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
	_ resource.Resource                = (*teamMembershipResource)(nil)
	_ resource.ResourceWithConfigure   = (*teamMembershipResource)(nil)
	_ resource.ResourceWithImportState = (*teamMembershipResource)(nil)
)

func NewTeamMembershipResource() resource.Resource { return &teamMembershipResource{} }

type teamMembershipResource struct{ c *client.Client }

type teamMembershipModel struct {
	ID       types.String `tfsdk:"id"`
	TeamID   types.String `tfsdk:"team_id"`
	MemberID types.String `tfsdk:"member_id"`
}

func (r *teamMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (r *teamMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Adds a member to a team. The composite ID is `<team_id>:<member_id>`. " +
			"Sigma has no single-membership read endpoint, so each refresh consults the team's full roster; the roster is listed once per team per Terraform run and reused across all of that team's memberships. For very large rosters (hundreds–thousands of members) prefer managing the set in the Sigma UI, as one `sigma_team_membership` resource per member is still one tracked resource each.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Composite ID `<team_id>:<member_id>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"team_id": schema.StringAttribute{
				Required:      true,
				Description:   "Team UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"member_id": schema.StringAttribute{
				Required:      true,
				Description:   "Member UUID.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *teamMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *teamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	teamID := plan.TeamID.ValueString()
	memberID := plan.MemberID.ValueString()
	body := teamMembersPatchReq{Add: []string{memberID}}
	if err := r.c.Do(ctx, "PATCH", "/v2/teams/"+client.EscapeID(teamID)+"/members", body, nil); err != nil {
		resp.Diagnostics.AddError("Add team member failed", err.Error())
		return
	}
	r.c.InvalidateTeamRoster(teamID)
	plan.ID = types.StringValue(teamID + ":" + memberID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *teamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	teamID := state.TeamID.ValueString()
	memberID := state.MemberID.ValueString()
	// API has no direct "get one member" endpoint; the roster must be listed and
	// filtered. The list is memoized per team for this run (see
	// client.TeamMemberIDs) so N memberships on one team cost a single roster
	// list, not N — the fix for issue #24's O(N²) blow-up.
	roster, err := r.c.TeamMemberIDs(teamID, func() ([]string, error) {
		members, lerr := client.ListAllPaged[teamMember](ctx, r.c, "/v2/teams/"+client.EscapeID(teamID)+"/members", url.Values{})
		if lerr != nil {
			return nil, lerr
		}
		ids := make([]string, len(members))
		for i, m := range members {
			ids[i] = m.UserID
		}
		return ids, nil
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List team members failed", err.Error())
		return
	}
	if _, ok := roster[memberID]; ok {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	// Membership no longer exists.
	resp.State.RemoveResource(ctx)
}

func (r *teamMembershipResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace conceptually (composite ID pieces);
	// nothing to update here. terraform-plugin-framework still requires the method.
}

func (r *teamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	teamID := state.TeamID.ValueString()
	body := teamMembersPatchReq{Remove: []string{state.MemberID.ValueString()}}
	if err := r.c.Do(ctx, "PATCH", "/v2/teams/"+client.EscapeID(teamID)+"/members", body, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Remove team member failed", err.Error())
		return
	}
	r.c.InvalidateTeamRoster(teamID)
}

func (r *teamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID",
			"Expected `<team_id>:<member_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("team_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("member_id"), parts[1])...)
}
