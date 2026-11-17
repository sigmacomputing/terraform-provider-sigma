// Package deploymentpolicies implements sigma_deployment_policy and its file/tenant associations.
package deploymentpolicies

// deploymentPolicyResp is returned by GET/PATCH /v2/deploymentPolicies/{id}.
type deploymentPolicyResp struct {
	DeploymentPolicyID       string   `json:"deploymentPolicyId"`
	Name                     string   `json:"name"`
	NameInTenant             string   `json:"nameInTenant"`
	VersionTagID             *string  `json:"versionTagId"`
	SourceSwapPolicies       []string `json:"sourceSwapPolicies"`
	CopyInputTableData       bool     `json:"copyInputTableData"`
	UseDependenciesWorkspace bool     `json:"useDependenciesWorkspace"`
}

type createDeploymentPolicyReq struct {
	Name                     string   `json:"name"`
	VersionTagID             *string  `json:"versionTagId,omitempty"`
	SourceSwapPolicies       []string `json:"sourceSwapPolicies,omitempty"`
	NameInTenant             *string  `json:"nameInTenant,omitempty"`
	CopyInputTableData       *bool    `json:"copyInputTableData,omitempty"`
	UseDependenciesWorkspace *bool    `json:"useDependenciesWorkspace,omitempty"`
}

type createDeploymentPolicyResp struct {
	DeploymentPolicyID string `json:"deploymentPolicyId"`
}

// versionTagId is omitted — not patchable, so it's RequiresReplace instead.
type patchDeploymentPolicyReq struct {
	Name                     *string   `json:"name,omitempty"`
	NameInTenant             *string   `json:"nameInTenant,omitempty"`
	SourceSwapPolicies       *[]string `json:"sourceSwapPolicies,omitempty"`
	CopyInputTableData       *bool     `json:"copyInputTableData,omitempty"`
	UseDependenciesWorkspace *bool     `json:"useDependenciesWorkspace,omitempty"`
}

// deploymentPolicyFileEntry is one entry of GET /v2/deploymentPolicies/{id}/files.
type deploymentPolicyFileEntry struct {
	InodeID            string `json:"inodeId"`
	DeploymentPolicyID string `json:"deploymentPolicyId"`
}

type addFilesReq struct {
	InodeIDs []string `json:"inodeIds"`
}

// GET on this path returns a bare []string, not an object — see ListAllPaged[string] usage.
type addTenantReq struct {
	TenantOrganizationID string `json:"tenantOrganizationId"`
}

// tenantOrgEntry: discovery list of tenant orgs from GET /v2/deploymentPolicies/tenants.
type tenantOrgEntry struct {
	TenantOrganizationID   string `json:"tenantOrganizationId"`
	TenantOrganizationSlug string `json:"tenantOrganizationSlug"`
	TenantOrganizationName string `json:"tenantOrganizationName"`
}
