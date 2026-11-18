package connections

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
	_ resource.Resource                = (*connectionGrantResource)(nil)
	_ resource.ResourceWithConfigure   = (*connectionGrantResource)(nil)
	_ resource.ResourceWithImportState = (*connectionGrantResource)(nil)
)

func NewGrantResource() resource.Resource { return &connectionGrantResource{} }

type connectionGrantResource struct{ c *client.Client }

type connectionGrantModel struct {
	ID           types.String `tfsdk:"id"`
	ConnectionID types.String `tfsdk:"connection_id"`
	GrantID      types.String `tfsdk:"grant_id"`
	MemberID     types.String `tfsdk:"member_id"`
	TeamID       types.String `tfsdk:"team_id"`
	Permission   types.String `tfsdk:"permission"`
}

func (r *connectionGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection_grant"
}

func (r *connectionGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a connection-level permission to either a member or a team. Composite ID `<connection_id>:<grant_id>`.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"connection_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"grant_id":      schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"member_id":     schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "Exactly one of `member_id` or `team_id` must be set."},
			"team_id":       schema.StringAttribute{Optional: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "Exactly one of `member_id` or `team_id` must be set."},
			"permission": schema.StringAttribute{Required: true, Description: "Either `usage` or `annotate`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		},
	}
}

func (r *connectionGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *connectionGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectionGrantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasMember := !plan.MemberID.IsNull() && !plan.MemberID.IsUnknown() && plan.MemberID.ValueString() != ""
	hasTeam := !plan.TeamID.IsNull() && !plan.TeamID.IsUnknown() && plan.TeamID.ValueString() != ""
	if hasMember == hasTeam {
		resp.Diagnostics.AddError("Invalid grantee", "Set exactly one of `member_id` or `team_id`.")
		return
	}
	g := grantee{}
	if hasMember {
		g.MemberID = plan.MemberID.ValueString()
	} else {
		g.TeamID = plan.TeamID.ValueString()
	}
	body := grantsBody{Grants: []grantSpec{{Grantee: g, Permission: plan.Permission.ValueString()}}}

	connID := plan.ConnectionID.ValueString()
	listPath := "/v2/connections/" + client.EscapeID(connID) + "/grants"

	matches := func(gv connectionGrant) bool {
		granteeOK := (hasMember && gv.MemberID != nil && *gv.MemberID == g.MemberID) ||
			(hasTeam && gv.TeamID != nil && *gv.TeamID == g.TeamID)
		return granteeOK && gv.Permission == plan.Permission.ValueString()
	}
	record := func(grantID string) {
		plan.GrantID = types.StringValue(grantID)
		plan.ID = types.StringValue(connID + ":" + grantID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}

	// Sigma's POST returns an empty body — we must list afterwards to discover
	// the new grantId. Capture the existing grants beforehand so we can tell
	// the new one apart from any pre-existing duplicate.
	before, err := client.ListAllPaged[connectionGrant](ctx, r.c, listPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Pre-create grant listing failed", err.Error())
		return
	}
	existing := make(map[string]struct{}, len(before))
	for _, gv := range before {
		// Adopt an identical pre-existing grant (e.g. an orphan from a prior
		// partially-failed apply) instead of POSTing a duplicate, so re-apply
		// converges.
		if matches(gv) {
			record(gv.GrantID)
			return
		}
		existing[gv.GrantID] = struct{}{}
	}

	defer client.InvalidateList(r.c, connGrantsKey(connID))
	if err := r.c.Do(ctx, "POST", listPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Create connection grant failed", err.Error())
		return
	}

	// The grant list is eventually consistent: re-list with backoff until the
	// new grant appears rather than erroring (and orphaning it) on the first miss.
	var newGrantID string
	err = client.Eventually(ctx, func() (bool, error) {
		err := client.ForEachPaged(ctx, r.c, listPath, nil, func(gv connectionGrant) (bool, error) {
			if _, seen := existing[gv.GrantID]; !seen && matches(gv) {
				newGrantID = gv.GrantID
				return true, nil
			}
			return false, nil
		})
		return newGrantID != "", err
	})
	if err != nil {
		granteeKind, granteeID := "team", g.TeamID
		if hasMember {
			granteeKind, granteeID = "member", g.MemberID
		}
		resp.Diagnostics.AddError(
			"Could not locate created grant",
			fmt.Sprintf("Posted a grant for %s=%s permission=%s but it does not appear in the post-create grant list after retrying: %s",
				granteeKind, granteeID, plan.Permission.ValueString(), err.Error()),
		)
		return
	}
	record(newGrantID)
}

func (r *connectionGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectionGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	connID := state.ConnectionID.ValueString()
	grantID := state.GrantID.ValueString()
	// Memoize the connection's grant list for the run so refreshing N grants on
	// one connection costs one list, not N (the issue #24 O(N²) shape).
	all, err := client.CachedList(r.c, connGrantsKey(connID), func() ([]connectionGrant, error) {
		return client.ListAllPaged[connectionGrant](ctx, r.c, "/v2/connections/"+client.EscapeID(connID)+"/grants", url.Values{})
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List connection grants failed", err.Error())
		return
	}
	for _, g := range all {
		if g.GrantID == grantID {
			state.Permission = types.StringValue(g.Permission)
			if g.MemberID != nil {
				state.MemberID = types.StringValue(*g.MemberID)
			}
			if g.TeamID != nil {
				state.TeamID = types.StringValue(*g.TeamID)
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *connectionGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes (connection_id, member_id, team_id, permission) are
	// RequiresReplace — Sigma's API has no PATCH for a single grant, so any
	// change destroys + recreates the grant. Update is therefore never invoked
	// for a real change and is a safe no-op.
}

func (r *connectionGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectionGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, connGrantsKey(state.ConnectionID.ValueString()))
	if err := r.c.Do(ctx, "DELETE", "/v2/connections/"+client.EscapeID(state.ConnectionID.ValueString())+"/grants/"+client.EscapeID(state.GrantID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete connection grant failed", err.Error())
	}
}

// connGrantsKey is the per-run CachedList key for a connection's grant list.
func connGrantsKey(connID string) string { return "conn-grants:" + connID }

func (r *connectionGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<connection_id>:<grant_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("connection_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("grant_id"), parts[1])...)
}
