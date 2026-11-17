package apiconnectors

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
	_ resource.Resource                   = (*connectorGrantResource)(nil)
	_ resource.ResourceWithConfigure      = (*connectorGrantResource)(nil)
	_ resource.ResourceWithImportState    = (*connectorGrantResource)(nil)
	_ resource.ResourceWithValidateConfig = (*connectorGrantResource)(nil)
)

// Sigma stores the UI's "Can use" as `view` and "Can manage" as `edit` (not the connection-style `usage`, which /v2/grants rejects).
const defaultConnectorPermission = "view"

var connectorPermissions = map[string]bool{"view": true, "edit": true}

func NewConnectorGrantResource() resource.Resource { return &connectorGrantResource{} }

type connectorGrantResource struct{ c *client.Client }

type connectorGrantModel struct {
	ID             types.String `tfsdk:"id"`
	APIConnectorID types.String `tfsdk:"api_connector_id"`
	GrantID        types.String `tfsdk:"grant_id"`
	MemberID       types.String `tfsdk:"member_id"`
	TeamID         types.String `tfsdk:"team_id"`
	Permission     types.String `tfsdk:"permission"`
}

func (r *connectorGrantResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_connector_grant"
}

func (r *connectorGrantResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Description: "Grants a member or team access to an API connector, via the generic `/v2/grants` endpoint with the connector ID as the `inodeId`. " +
			"Users also need an account type with **Trigger API actions** to run the connector from a workbook. Composite ID `<api_connector_id>:<grant_id>`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"api_connector_id": schema.StringAttribute{Required: true, PlanModifiers: replace,
				Description: "ID of the `sigma_api_connector` to grant access on."},
			"grant_id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"member_id": schema.StringAttribute{Optional: true, PlanModifiers: replace,
				Description: "Exactly one of `member_id` or `team_id` must be set."},
			"team_id": schema.StringAttribute{Optional: true, PlanModifiers: replace,
				Description: "Exactly one of `member_id` or `team_id` must be set."},
			"permission": schema.StringAttribute{Optional: true, Computed: true, PlanModifiers: replace,
				Default:     stringdefault.StaticString(defaultConnectorPermission),
				Description: "`view` (the Sigma UI's **Can use**, the default) or `edit` (**Can manage**)."},
		},
	}
}

func (r *connectorGrantResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var perm types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("permission"), &perm)...)
	if perm.IsNull() || perm.IsUnknown() || connectorPermissions[perm.ValueString()] {
		return
	}
	resp.Diagnostics.AddAttributeError(path.Root("permission"), "Invalid permission",
		fmt.Sprintf("permission must be `view` (Can use) or `edit` (Can manage), got %q.", perm.ValueString()))
}

func (r *connectorGrantResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *connectorGrantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectorGrantModel
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
	g := grantGrantee{}
	if hasMember {
		g.MemberID = plan.MemberID.ValueString()
	} else {
		g.TeamID = plan.TeamID.ValueString()
	}
	connID := plan.APIConnectorID.ValueString()
	perm := plan.Permission.ValueString()

	record := func(grantID string) {
		plan.GrantID = types.StringValue(grantID)
		plan.ID = types.StringValue(connID + ":" + grantID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}

	// Adopt an identical existing grant (e.g. one clicked in via the UI) instead of POSTing a duplicate.
	existing, err := client.ListAllPaged[inodeGrant](ctx, r.c, "/v2/grants", url.Values{"inodeId": {connID}, "directGrantsOnly": {"true"}})
	if err != nil {
		resp.Diagnostics.AddError("Pre-create grant listing failed", err.Error())
		return
	}
	for _, x := range existing {
		if x.matches(g, perm) {
			record(x.GrantID)
			return
		}
	}

	var got inodeGrant
	body := createGrantReq{Grantee: g, Permission: perm, InodeID: connID}
	if err := r.c.Do(ctx, "POST", "/v2/grants", body, &got); err != nil {
		resp.Diagnostics.AddError("Create API connector grant failed", err.Error())
		return
	}
	if got.GrantID == "" {
		resp.Diagnostics.AddError("Create API connector grant failed", "POST /v2/grants returned no grantId")
		return
	}
	record(got.GrantID)
}

func (r *connectorGrantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectorGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got inodeGrant
	err := r.c.Do(ctx, "GET", "/v2/grants/"+client.EscapeID(state.GrantID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read API connector grant failed", err.Error())
		return
	}
	// got.InodeID is Sigma's internal inode ID, not the apiConnectorId, so api_connector_id is kept from state.
	state.Permission = types.StringValue(got.Permission)
	if got.MemberID != nil && *got.MemberID != "" {
		state.MemberID = types.StringValue(*got.MemberID)
	}
	if got.TeamID != nil && *got.TeamID != "" {
		state.TeamID = types.StringValue(*got.TeamID)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is a no-op: every attribute is RequiresReplace.
func (r *connectorGrantResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (r *connectorGrantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectorGrantModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.Do(ctx, "DELETE", "/v2/grants/"+client.EscapeID(state.GrantID.ValueString()), nil, nil)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete API connector grant failed", err.Error())
	}
}

func (r *connectorGrantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<api_connector_id>:<grant_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("api_connector_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("grant_id"), parts[1])...)
}
