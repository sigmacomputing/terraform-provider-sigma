package workspaces

import "time"

type workspace struct {
	WorkspaceID    string    `json:"workspaceId"`
	WorkspaceURLID string    `json:"workspaceUrlId"`
	Name           string    `json:"name"`
	CreatedBy      string    `json:"createdBy"`
	UpdatedBy      string    `json:"updatedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type createWorkspaceReq struct {
	Name         string `json:"name"`
	NoDuplicates *bool  `json:"noDuplicates,omitempty"`
}

type patchWorkspaceReq struct {
	Name string `json:"name"`
}

// grantee mirrors Sigma's oneOf — exactly one of MemberID/TeamID must be set.
type grantee struct {
	MemberID string `json:"memberId,omitempty"`
	TeamID   string `json:"teamId,omitempty"`
}

type grantSpec struct {
	Grantee    grantee `json:"grantee"`
	Permission string  `json:"permission"`
}

type grantsBody struct {
	Grants []grantSpec `json:"grants"`
}

type workspaceGrant struct {
	GrantID    string  `json:"grantId"`
	InodeID    string  `json:"inodeId"`
	Permission string  `json:"permission"`
	MemberID   *string `json:"memberId,omitempty"`
	TeamID     *string `json:"teamId,omitempty"`
}
