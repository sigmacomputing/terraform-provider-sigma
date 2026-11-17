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
	_ resource.Resource                = (*pathGrantResource)(nil)
	_ resource.ResourceWithConfigure   = (*pathGrantResource)(nil)
	_ resource.ResourceWithImportState = (*pathGrantResource)(nil)
)

func NewPathGrantResource() resource.Resource { return &pathGrantResource{} }

type pathGrantResource struct{ c *client.Client }

type pathGrantModel struct {
	ID               types.String `tfsdk:"id"`
	ConnectionPathID types.String `tfsdk:"connection_path_id"`
	GrantID          types.String `tfsdk:"grant_id"`
	MemberID         types.String `tfsdk:"member_id"`
	TeamID           types.String `tfsdk:"team_id"`
	Permission       types.String `tfsdk:"permission"`
}

func (r *pathGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connection_path_grant"
}

func (r *pathGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a permission on a specific connection-path node (database/schema/table). Composite ID `<connection_path_id>:<grant_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"connection_path_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Connection path UUID. Use the `sigma_connection_path` data source to look up paths."},
			"grant_id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"member_id": schema.StringAttribute{Optional: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Exactly one of `member_id` or `team_id` must be set."},
			"team_id": schema.StringAttribute{Optional: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Exactly one of `member_id` or `team_id` must be set."},
			"permission": schema.StringAttribute{Required: true, Description: "Either `usage` or `annotate`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
		},
	}
}

func (r *pathGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pathGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pathGrantModel
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
	pathID := plan.ConnectionPathID.ValueString()
	listPath := "/v2/connections/paths/" + client.EscapeID(pathID) + "/grants"

	matches := func(gv connectionGrant) bool {
		granteeOK := (hasMember && gv.MemberID != nil && *gv.MemberID == g.MemberID) ||
			(hasTeam && gv.TeamID != nil && *gv.TeamID == g.TeamID)
		return granteeOK && gv.Permission == plan.Permission.ValueString()
	}
	record := func(grantID string) {
		plan.GrantID = types.StringValue(grantID)
		plan.ID = types.StringValue(pathID + ":" + grantID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}

	// POST returns empty body; diff before/after to locate the new grantId.
	before, err := client.ListAllPaged[connectionGrant](ctx, r.c, listPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Pre-create path grant listing failed", err.Error())
		return
	}
	existing := make(map[string]struct{}, len(before))
	for _, gv := range before {
		// Adopt an identical pre-existing grant (e.g. an orphan from a prior
		// partially-failed apply) instead of POSTing a duplicate.
		if matches(gv) {
			record(gv.GrantID)
			return
		}
		existing[gv.GrantID] = struct{}{}
	}

	defer client.InvalidateList(r.c, pathGrantsKey(pathID))
	if err := r.c.Do(ctx, "POST", listPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Create connection path grant failed", err.Error())
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
		resp.Diagnostics.AddError(
			"Could not locate created path grant",
			fmt.Sprintf("Posted a grant for path %s but it does not appear in the post-create grant list after retrying: %s", pathID, err.Error()),
		)
		return
	}
	record(newGrantID)
}

func (r *pathGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pathGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pathID := state.ConnectionPathID.ValueString()
	grantID := state.GrantID.ValueString()
	all, err := client.CachedList(r.c, pathGrantsKey(pathID), func() ([]connectionGrant, error) {
		return client.ListAllPaged[connectionGrant](ctx, r.c, "/v2/connections/paths/"+client.EscapeID(pathID)+"/grants", url.Values{})
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List path grants failed", err.Error())
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

func (r *pathGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace (no PATCH for a single path grant), so a
	// permission change destroys + recreates the grant. Update is a safe no-op.
}

func (r *pathGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pathGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, pathGrantsKey(state.ConnectionPathID.ValueString()))
	if err := r.c.Do(ctx, "DELETE", "/v2/connections/paths/"+client.EscapeID(state.ConnectionPathID.ValueString())+"/grants/"+client.EscapeID(state.GrantID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete path grant failed", err.Error())
	}
}

// pathGrantsKey is the per-run CachedList key for a connection path's grant list.
func pathGrantsKey(pathID string) string { return "path-grants:" + pathID }

func (r *pathGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<connection_path_id>:<grant_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("connection_path_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("grant_id"), parts[1])...)
}
