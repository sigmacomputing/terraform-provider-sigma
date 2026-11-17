package workspaces

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
	_ resource.Resource                = (*grantResource)(nil)
	_ resource.ResourceWithConfigure   = (*grantResource)(nil)
	_ resource.ResourceWithImportState = (*grantResource)(nil)
)

func NewGrantResource() resource.Resource { return &grantResource{} }

type grantResource struct{ c *client.Client }

type grantModel struct {
	ID          types.String `tfsdk:"id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	GrantID     types.String `tfsdk:"grant_id"`
	MemberID    types.String `tfsdk:"member_id"`
	TeamID      types.String `tfsdk:"team_id"`
	Permission  types.String `tfsdk:"permission"`
}

func (r *grantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workspace_grant"
}

func (r *grantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Grants a workspace permission to a member or team. Composite ID `<workspace_id>:<grant_id>`. Permission enum: `view`, `explore`, `organize`, `edit`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"workspace_id": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"grant_id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"member_id": schema.StringAttribute{Optional: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Exactly one of `member_id` or `team_id` must be set."},
			"team_id": schema.StringAttribute{Optional: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Exactly one of `member_id` or `team_id` must be set."},
			"permission": schema.StringAttribute{Required: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description: "One of `view`, `explore`, `organize`, `edit`. These map to the Sigma Share-dialog labels: " +
					"`view`=Can view, `explore`=Can explore, `organize`=**Can contribute**, `edit`=**Can manage**. " +
					"Note the last two are easy to get backwards — `organize` is the mid-tier \"Can contribute\", `edit` is the top-tier \"Can manage\"."},
		},
	}
}

func (r *grantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *grantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan grantModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasMember := !plan.MemberID.IsNull() && plan.MemberID.ValueString() != ""
	hasTeam := !plan.TeamID.IsNull() && plan.TeamID.ValueString() != ""
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
	wsID := plan.WorkspaceID.ValueString()
	listPath := "/v2/workspaces/" + client.EscapeID(wsID) + "/grants"

	matches := func(gv workspaceGrant) bool {
		granteeOK := (hasMember && gv.MemberID != nil && *gv.MemberID == g.MemberID) ||
			(hasTeam && gv.TeamID != nil && *gv.TeamID == g.TeamID)
		return granteeOK && gv.Permission == plan.Permission.ValueString()
	}
	record := func(grantID string) {
		plan.GrantID = types.StringValue(grantID)
		plan.ID = types.StringValue(wsID + ":" + grantID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}

	// POST returns an empty body, so the new grantId is discovered by listing.
	before, err := client.ListAllPaged[workspaceGrant](ctx, r.c, listPath, nil)
	if err != nil {
		resp.Diagnostics.AddError("Pre-create grant listing failed", err.Error())
		return
	}
	existing := make(map[string]struct{}, len(before))
	for _, x := range before {
		// Adopt an identical grant that already exists (e.g. an orphan stranded
		// by a prior partially-failed apply) instead of POSTing a duplicate, so
		// re-apply converges rather than re-failing.
		if matches(x) {
			record(x.GrantID)
			return
		}
		existing[x.GrantID] = struct{}{}
	}

	body := grantsBody{Grants: []grantSpec{{Grantee: g, Permission: plan.Permission.ValueString()}}}
	defer client.InvalidateList(r.c, wsGrantsKey(wsID))
	if err := r.c.Do(ctx, "POST", listPath, body, nil); err != nil {
		resp.Diagnostics.AddError("Create workspace grant failed", err.Error())
		return
	}

	// The grant list is eventually consistent, so re-list with backoff until the
	// newly created grant appears rather than erroring (and orphaning it) on the
	// first miss — which under a concurrent multi-grant apply hit later grants.
	var newGrantID string
	err = client.Eventually(ctx, func() (bool, error) {
		err := client.ForEachPaged(ctx, r.c, listPath, nil, func(gv workspaceGrant) (bool, error) {
			if _, seen := existing[gv.GrantID]; !seen && matches(gv) {
				newGrantID = gv.GrantID
				return true, nil
			}
			return false, nil
		})
		return newGrantID != "", err
	})
	if err != nil {
		resp.Diagnostics.AddError("Could not locate created grant",
			"POST succeeded but no matching grant appeared in the workspace grant list after retrying: "+err.Error())
		return
	}
	record(newGrantID)
}

func (r *grantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state grantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wsID := state.WorkspaceID.ValueString()
	grantID := state.GrantID.ValueString()
	all, err := client.CachedList(r.c, wsGrantsKey(wsID), func() ([]workspaceGrant, error) {
		return client.ListAllPaged[workspaceGrant](ctx, r.c, "/v2/workspaces/"+client.EscapeID(wsID)+"/grants", url.Values{})
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List workspace grants failed", err.Error())
		return
	}
	for _, gv := range all {
		if gv.GrantID == grantID {
			state.Permission = types.StringValue(gv.Permission)
			if gv.MemberID != nil {
				state.MemberID = types.StringValue(*gv.MemberID)
			}
			if gv.TeamID != nil {
				state.TeamID = types.StringValue(*gv.TeamID)
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *grantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All attributes are RequiresReplace (no PATCH for a single grant), so a
	// permission change destroys + recreates the grant. Update is never invoked
	// for a real change and is a safe no-op.
}

func (r *grantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state grantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	defer client.InvalidateList(r.c, wsGrantsKey(state.WorkspaceID.ValueString()))
	url := "/v2/workspaces/" + client.EscapeID(state.WorkspaceID.ValueString()) + "/grants/" + client.EscapeID(state.GrantID.ValueString())
	if err := r.c.Do(ctx, "DELETE", url, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete workspace grant failed", err.Error())
	}
}

// wsGrantsKey is the per-run CachedList key for a workspace's grant list.
func wsGrantsKey(wsID string) string { return "ws-grants:" + wsID }

func (r *grantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<workspace_id>:<grant_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("grant_id"), parts[1])...)
}
