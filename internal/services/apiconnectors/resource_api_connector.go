package apiconnectors

import (
	"context"
	"encoding/json"
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
	_ resource.Resource                = (*connectorResource)(nil)
	_ resource.ResourceWithConfigure   = (*connectorResource)(nil)
	_ resource.ResourceWithImportState = (*connectorResource)(nil)
)

func NewConnectorResource() resource.Resource { return &connectorResource{} }

type connectorResource struct{ c *client.Client }

type connectorModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ParamsJSON  types.String `tfsdk:"params_json"`
	ConfigJSON  types.String `tfsdk:"config_json"`
	AuthID      types.String `tfsdk:"auth_id"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *connectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_connector"
}

func (r *connectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "An API connector: defines how Sigma calls an external HTTP endpoint from within a workbook. " +
			"`params_json`/`config_json` are forwarded verbatim (opaque JSON) and are **not** refreshed from GET, so there is no drift detection on those fields (same trade-off as `sigma_connection.details_json`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"name":        schema.StringAttribute{Required: true, Description: "Display name of the connector."},
			"description": schema.StringAttribute{Optional: true, Computed: true, Description: "Free-text description."},
			"params_json": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "JSON-encoded `params` object per Sigma's POST /v2/api-connectors schema: `method`, `url`, `headers`, `pathParams`, `queryParams`, `body`. The `url` host must match the credential's allowlist. Sensitive, since headers and query params can carry keys.",
			},
			"config_json": schema.StringAttribute{
				Optional:    true,
				Description: "JSON-encoded optional `config` object per Sigma's POST /v2/api-connectors schema.",
			},
			"auth_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of the `sigma_api_connector_credential` used for outbound auth. Omit for unauthenticated connectors.",
			},
			"created_at": schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"updated_at": schema.StringAttribute{Computed: true},
		},
	}
}

func (r *connectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *connectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan connectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, ok := r.buildBody(plan, resp.Diagnostics.AddAttributeError)
	if resp.Diagnostics.HasError() || !ok {
		return
	}
	var got apiConnector
	if err := r.c.Do(ctx, "POST", "/v2/api-connectors", body, &got); err != nil {
		resp.Diagnostics.AddError("Create API connector failed", err.Error())
		return
	}
	r.apply(&got, &plan) // preserves params_json/config_json from plan
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var got apiConnector
	err := r.c.Do(ctx, "GET", "/v2/api-connectors/"+client.EscapeID(state.ID.ValueString()), nil, &got)
	if client.IsNotFound(err) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Read API connector failed", err.Error())
		return
	}
	r.apply(&got, &state) // params_json/config_json retained from state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *connectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state connectorModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body, ok := r.buildBody(plan, resp.Diagnostics.AddAttributeError)
	if resp.Diagnostics.HasError() || !ok {
		return
	}
	var got apiConnector
	if err := r.c.Do(ctx, "PATCH", "/v2/api-connectors/"+client.EscapeID(state.ID.ValueString()), body, &got); err != nil {
		resp.Diagnostics.AddError("Update API connector failed", err.Error())
		return
	}
	r.apply(&got, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *connectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state connectorModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.Do(ctx, "DELETE", "/v2/api-connectors/"+client.EscapeID(state.ID.ValueString()), nil, nil); err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete API connector failed", err.Error())
	}
}

func (r *connectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// buildBody assembles the create/patch request. params_json/config_json are parsed
// and sent verbatim. Returns ok=false (with a diagnostic appended) on invalid JSON.
func (r *connectorResource) buildBody(plan connectorModel, addErr func(path.Path, string, string)) (map[string]any, bool) {
	var params any
	if err := json.Unmarshal([]byte(plan.ParamsJSON.ValueString()), &params); err != nil {
		addErr(path.Root("params_json"), "Invalid JSON", err.Error())
		return nil, false
	}
	body := map[string]any{
		"name":   plan.Name.ValueString(),
		"params": params,
	}
	if !plan.ConfigJSON.IsNull() && !plan.ConfigJSON.IsUnknown() && plan.ConfigJSON.ValueString() != "" {
		var config any
		if err := json.Unmarshal([]byte(plan.ConfigJSON.ValueString()), &config); err != nil {
			addErr(path.Root("config_json"), "Invalid JSON", err.Error())
			return nil, false
		}
		body["config"] = config
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		body["description"] = plan.Description.ValueString()
	}
	if !plan.AuthID.IsNull() && !plan.AuthID.IsUnknown() && plan.AuthID.ValueString() != "" {
		body["authId"] = plan.AuthID.ValueString()
	}
	return body, true
}

// apply copies server-returned fields into dst, leaving params_json/config_json
// untouched (write-only/opaque).
func (r *connectorResource) apply(c *apiConnector, dst *connectorModel) {
	dst.ID = types.StringValue(c.APIConnectorID)
	dst.Name = types.StringValue(c.Name)
	dst.Description = types.StringValue(c.Description)
	if c.AuthID != nil {
		dst.AuthID = types.StringValue(*c.AuthID)
	} else {
		dst.AuthID = types.StringNull()
	}
	dst.CreatedAt = types.StringValue(c.CreatedAt.Format(tsLayout))
	dst.UpdatedAt = types.StringValue(c.UpdatedAt.Format(tsLayout))
}
