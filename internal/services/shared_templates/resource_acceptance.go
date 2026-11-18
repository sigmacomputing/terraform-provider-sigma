// Package shared_templates implements sigma_shared_template_acceptance +
// sigma_shared_templates.
//
// Sigma's cross-org template-share API has 3 endpoints:
//
//	GET    /v2/shared_templates/shared_with_you   — list shares
//	POST   /v2/shared_templates/accept             — accept a pending share
//	DELETE /v2/shared_templates/{shareId}          — remove a share
//
// We model the accept-then-delete lifecycle as a single resource: Create
// runs the POST, Delete runs the DELETE. There's no Update path (accept
// is one-shot), so any input change forces replace.
package shared_templates

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ resource.Resource                = (*acceptanceResource)(nil)
	_ resource.ResourceWithConfigure   = (*acceptanceResource)(nil)
	_ resource.ResourceWithImportState = (*acceptanceResource)(nil)
)

func NewResource() resource.Resource { return &acceptanceResource{} }

type acceptanceResource struct{ c *client.Client }

type acceptanceModel struct {
	ID      types.String `tfsdk:"id"`
	ShareID types.String `tfsdk:"share_id"`
}

func (r *acceptanceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_template_acceptance"
}

func (r *acceptanceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Acceptance of a workbook template shared with this org. Create runs `POST /v2/shared_templates/accept`; destroy runs `DELETE /v2/shared_templates/{shareId}`. Discover pending shares via the `data.sigma_shared_templates` data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"share_id": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Identifier of the pending share to accept.",
			},
		},
	}
}

func (r *acceptanceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *acceptanceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan acceptanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := map[string]any{"shareId": plan.ShareID.ValueString()}
	if err := r.c.Do(ctx, "POST", "/v2/shared_templates/accept", body, nil); err != nil {
		resp.Diagnostics.AddError("Accept shared template failed", err.Error())
		return
	}
	plan.ID = plan.ShareID
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *acceptanceResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
	// No-op: there's no GET-by-ID for a specific share. Listing is best-effort
	// via the data source. State preserves what we recorded at Create.
}

func (r *acceptanceResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// share_id is RequiresReplace; Update is unreachable.
}

func (r *acceptanceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state acceptanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	url := "/v2/shared_templates/" + client.EscapeID(state.ShareID.ValueString())
	if err := r.c.Do(ctx, "DELETE", url, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete shared template share failed", err.Error())
	}
}

func (r *acceptanceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("share_id"), req.ID)...)
}
