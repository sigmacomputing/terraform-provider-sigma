package deploymentpolicies

import (
	"context"
	"fmt"
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
	_ resource.Resource                = (*deploymentPolicyFileResource)(nil)
	_ resource.ResourceWithConfigure   = (*deploymentPolicyFileResource)(nil)
	_ resource.ResourceWithImportState = (*deploymentPolicyFileResource)(nil)
)

func NewFileResource() resource.Resource { return &deploymentPolicyFileResource{} }

type deploymentPolicyFileResource struct{ c *client.Client }

type deploymentPolicyFileModel struct {
	ID                 types.String `tfsdk:"id"`
	DeploymentPolicyID types.String `tfsdk:"deployment_policy_id"`
	InodeID            types.String `tfsdk:"inode_id"`
}

func (r *deploymentPolicyFileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_deployment_policy_file"
}

func (r *deploymentPolicyFileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Attaches one file (workbook, data model, report, or folder inode) to a deployment policy. Composite ID `<deployment_policy_id>:<inode_id>`. Use one resource instance per file (e.g. with `for_each`) to attach several.",
		Attributes: map[string]schema.Attribute{
			"id":                   schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"deployment_policy_id": schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}},
			"inode_id":             schema.StringAttribute{Required: true, PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()}, Description: "ID of the file/inode to attach."},
		},
	}
}

func (r *deploymentPolicyFileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *deploymentPolicyFileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan deploymentPolicyFileModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policyID := plan.DeploymentPolicyID.ValueString()
	inodeID := plan.InodeID.ValueString()
	listPath := "/v2/deploymentPolicies/" + client.EscapeID(policyID) + "/files"

	defer client.InvalidateList(r.c, deploymentPolicyFilesKey(policyID))
	if err := r.c.Do(ctx, "POST", listPath, addFilesReq{InodeIDs: []string{inodeID}}, nil); err != nil {
		resp.Diagnostics.AddError("Attach file to deployment policy failed", err.Error())
		return
	}
	// inodeId is client-supplied, not server-minted — just confirm it landed.
	err := client.Eventually(ctx, func() (bool, error) {
		all, err := client.ListAllPageToken[deploymentPolicyFileEntry](ctx, r.c, listPath, client.PageSize(500))
		if err != nil {
			return false, err
		}
		for _, f := range all {
			if f.InodeID == inodeID {
				return true, nil
			}
		}
		return false, nil
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Could not confirm file attachment",
			fmt.Sprintf("Attached inode %s to deployment policy %s but it does not appear in the post-attach file list after retrying: %s", inodeID, policyID, err.Error()),
		)
		return
	}
	plan.ID = types.StringValue(policyID + ":" + inodeID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *deploymentPolicyFileResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state deploymentPolicyFileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policyID := state.DeploymentPolicyID.ValueString()
	inodeID := state.InodeID.ValueString()
	all, err := client.CachedList(r.c, deploymentPolicyFilesKey(policyID), func() ([]deploymentPolicyFileEntry, error) {
		return client.ListAllPageToken[deploymentPolicyFileEntry](ctx, r.c, "/v2/deploymentPolicies/"+client.EscapeID(policyID)+"/files", client.PageSize(500))
	})
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("List deployment policy files failed", err.Error())
		return
	}
	for _, f := range all {
		if f.InodeID == inodeID {
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

func (r *deploymentPolicyFileResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// RequiresReplace on everything; no PATCH exists, so this is a safe no-op.
}

func (r *deploymentPolicyFileResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state deploymentPolicyFileModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	policyID := state.DeploymentPolicyID.ValueString()
	defer client.InvalidateList(r.c, deploymentPolicyFilesKey(policyID))
	p := "/v2/deploymentPolicies/" + client.EscapeID(policyID) + "/files/" + client.EscapeID(state.InodeID.ValueString())
	if err := r.c.Do(ctx, "DELETE", p, nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Detach file from deployment policy failed", err.Error())
	}
}

func (r *deploymentPolicyFileResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid import ID", "Expected `<deployment_policy_id>:<inode_id>`, got "+req.ID)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("deployment_policy_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("inode_id"), parts[1])...)
}

// deploymentPolicyFilesKey is the CachedList key for a policy's file associations.
func deploymentPolicyFilesKey(policyID string) string { return "deploy-policy-files:" + policyID }
