package connections

import "time"

type connection struct {
	OrganizationID string    `json:"organizationId"`
	ConnectionID   string    `json:"connectionId"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	IsSample       bool      `json:"isSample"`
	IsAuditLog     bool      `json:"isAuditLog"`
	UseOauth       bool      `json:"useOauth"`
	OauthClientID  string    `json:"oauthClientId,omitempty"`
	OauthJwkSetID  string    `json:"oauthJwkSetId,omitempty"`
	RoleSwitching  string    `json:"roleSwitching,omitempty"`
	LastActiveAt   time.Time `json:"lastActiveAt"`
	CreatedBy      string    `json:"createdBy"`
	UpdatedBy      string    `json:"updatedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// grantee carries either a memberId or a teamId per Sigma's oneOf shape.
// Exactly one of Member/Team must be set.
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

// connectionGrant is the response shape for GET .../grants entries.
type connectionGrant struct {
	GrantID    string  `json:"grantId"`
	Permission string  `json:"permission"`
	MemberID   *string `json:"memberId,omitempty"`
	TeamID     *string `json:"teamId,omitempty"`
}
