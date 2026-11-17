// Package plugins implements the sigma_plugin resource and the sigma_plugins
// data source, backed by Sigma's custom plugins API (/v2/plugins, beta).
package plugins

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
	_ resource.Resource                = (*pluginResource)(nil)
	_ resource.ResourceWithConfigure   = (*pluginResource)(nil)
	_ resource.ResourceWithImportState = (*pluginResource)(nil)
)

func NewResource() resource.Resource { return &pluginResource{} }

type pluginResource struct{ c *client.Client }

type pluginModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	URL         types.String `tfsdk:"url"`
	DevURL      types.String `tfsdk:"dev_url"`
	Type        types.String `tfsdk:"type"`
}

func (r *pluginResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_plugin"
}

func (r *pluginResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A Sigma custom plugin (beta API). Sigma loads the plugin from `url` when it is added to a workbook, and from `dev_url` when the plugin runs in development mode.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Unique identifier (UUID) of the custom plugin (`pluginId`).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Display name of the custom plugin.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Description of the custom plugin.",
			},
			"url": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Production URL where the plugin is hosted. Sigma loads the plugin from this URL when it is added to a workbook. Defaults to an empty string (no production URL set) when omitted at creation; removing it later keeps the current value, so set `\"\"` to clear it. Sigma's API can't change it in place, so changing it replaces the plugin, which gets a new `id`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"dev_url": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Development URL for the plugin. Sigma loads the plugin from this URL when it runs in development mode. Defaults to `http://localhost:5173` when omitted.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Plugin type. Currently always `element`.",
			},
		},
	}
}

func (r *pluginResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *pluginResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan pluginModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := createPluginReq{Name: plan.Name.ValueString()}
	if v := strPtrIfSet(plan.Description); v != nil {
		body.Description = v
	}
	if v := strPtrIfSet(plan.URL); v != nil {
		body.URL = v
	}
	if v := strPtrIfSet(plan.DevURL); v != nil {
		body.DevURL = v
	}
	var created plugin
	if err := r.c.Do(ctx, "POST", "/v2/plugins", body, &created); err != nil {
		resp.Diagnostics.AddError("Create plugin failed", err.Error())
		return
	}
	pluginToState(&created, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pluginResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state pluginModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got plugin
	if err := r.c.Do(ctx, "GET", "/v2/plugins/"+client.EscapeID(state.ID.ValueString()), nil, &got); err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read plugin failed", err.Error())
		return
	}
	pluginToState(&got, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *pluginResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state pluginModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	patch := patchPluginReq{}
	if !plan.Name.Equal(state.Name) {
		v := plan.Name.ValueString()
		patch.Name = &v
	}
	if !plan.Description.Equal(state.Description) {
		v := plan.Description.ValueString()
		patch.Description = &v
	}
	if !plan.DevURL.Equal(state.DevURL) {
		v := plan.DevURL.ValueString()
		patch.DevURL = &v
	}
	var updated plugin
	if err := r.c.Do(ctx, "PATCH", "/v2/plugins/"+client.EscapeID(state.ID.ValueString()), patch, &updated); err != nil {
		resp.Diagnostics.AddError("Update plugin failed", err.Error())
		return
	}
	pluginToState(&updated, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *pluginResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state pluginModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/plugins/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete plugin failed", err.Error())
	}
}

func (r *pluginResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// strPtrIfSet returns a pointer to the string value when the attribute is set
// (non-null, known), or nil so the field is omitted from the request body.
func strPtrIfSet(s types.String) *string {
	if s.IsNull() || s.IsUnknown() {
		return nil
	}
	v := s.ValueString()
	return &v
}

func pluginToState(p *plugin, dst *pluginModel) {
	dst.ID = types.StringValue(p.PluginID)
	dst.Name = types.StringValue(p.Name)
	dst.URL = types.StringValue(p.URL)
	dst.Type = types.StringValue(p.Type)
	dst.Description = strOrNull(p.Description)
	dst.DevURL = strOrNull(p.DevURL)
}

func strOrNull(p *string) types.String {
	if p == nil {
		return types.StringNull()
	}
	return types.StringValue(*p)
}
