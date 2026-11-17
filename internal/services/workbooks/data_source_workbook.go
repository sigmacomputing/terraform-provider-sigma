// Package workbooks implements workbook-family resources and data sources.
//
// Note: Sigma's public REST API has no PATCH or DELETE for workbooks
// themselves. Workbook content lifecycle is UI-only. This provider treats
// workbooks as data sources; the sub-resources (grant, schedule, embed,
// bookmark, version_tag) are managed via the API and are full TF resources.
package workbooks

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/sigmacomputing/terraform-provider-sigma/internal/client"
)

var (
	_ datasource.DataSource              = (*workbookDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*workbookDataSource)(nil)
)

func NewWorkbookDataSource() datasource.DataSource { return &workbookDataSource{} }

type workbookDataSource struct{ c *client.Client }

type workbookDSModel struct {
	ID            types.String `tfsdk:"id"`
	URLID         types.String `tfsdk:"url_id"`
	Name          types.String `tfsdk:"name"`
	URL           types.String `tfsdk:"url"`
	Path          types.String `tfsdk:"path"`
	LatestVersion types.Int64  `tfsdk:"latest_version"`
	OwnerID       types.String `tfsdk:"owner_id"`
}

func (d *workbookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_workbook"
}

func (d *workbookDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up a workbook by `id` (preferred) or by `path` (e.g. `My Documents/Reports/Q4`). Workbooks are not creatable via Sigma's REST API; create them in the UI and reference here.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Optional: true, Computed: true, Description: "Workbook UUID."},
			"url_id":         schema.StringAttribute{Computed: true},
			"name":           schema.StringAttribute{Computed: true},
			"url":            schema.StringAttribute{Computed: true},
			"path":           schema.StringAttribute{Optional: true, Computed: true, Description: "Slash-separated path."},
			"latest_version": schema.Int64Attribute{Computed: true},
			"owner_id":       schema.StringAttribute{Computed: true},
		},
	}
}

func (d *workbookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	d.c = c
}

func (d *workbookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data workbookDSModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasID := !data.ID.IsNull() && data.ID.ValueString() != ""
	hasPath := !data.Path.IsNull() && data.Path.ValueString() != ""
	if !hasID && !hasPath {
		resp.Diagnostics.AddError("Missing input", "Either `id` or `path` must be set.")
		return
	}

	if hasID {
		var got workbook
		if err := d.c.Do(ctx, "GET", "/v2/workbooks/"+client.EscapeID(data.ID.ValueString()), nil, &got); err != nil {
			resp.Diagnostics.AddError("Get workbook failed", err.Error())
			return
		}
		applyWorkbook(&got, &data)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	wanted := data.Path.ValueString()
	q := url.Values{}
	var found *workbook
	err := client.ForEachPaged[workbook](ctx, d.c, "/v2/workbooks", q, func(w workbook) (bool, error) {
		if strings.EqualFold(w.Path, wanted) || w.Path == wanted {
			found = &w
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		resp.Diagnostics.AddError("List workbooks failed", err.Error())
		return
	}
	if found == nil {
		resp.Diagnostics.AddError("Workbook not found", fmt.Sprintf("No workbook with path %q.", wanted))
		return
	}
	applyWorkbook(found, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func applyWorkbook(w *workbook, dst *workbookDSModel) {
	dst.ID = types.StringValue(w.WorkbookID)
	dst.URLID = types.StringValue(w.WorkbookURLID)
	dst.Name = types.StringValue(w.Name)
	dst.URL = types.StringValue(w.URL)
	dst.Path = types.StringValue(w.Path)
	dst.LatestVersion = types.Int64Value(int64(w.LatestVersion))
	dst.OwnerID = types.StringValue(w.OwnerID)
}
