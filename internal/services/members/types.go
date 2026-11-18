package members

import "time"

// member is the v2 API response shape for /v2/members/{memberId}.
type member struct {
	OrganizationID string    `json:"organizationId"`
	MemberID       string    `json:"memberId"`
	MemberType     string    `json:"memberType"`
	FirstName      string    `json:"firstName"`
	LastName       string    `json:"lastName"`
	Email          string    `json:"email"`
	ProfileImgURL  *string   `json:"profileImgUrl"`
	CreatedBy      string    `json:"createdBy"`
	UpdatedBy      string    `json:"updatedBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	HomeFolderID   string    `json:"homeFolderId"`
	UserKind       string    `json:"userKind"`
	Language       *string   `json:"language,omitempty"`
	FormatRegion   *string   `json:"formatRegion,omitempty"`
	IsArchived     *bool     `json:"isArchived,omitempty"`
	IsInactive     *bool     `json:"isInactive,omitempty"`
}

// createMemberReq is the POST /v2/members body. Only the fields TF cares about
// at create time; team adds are handled via sigma_team_membership.
type createMemberReq struct {
	Email        string  `json:"email"`
	FirstName    string  `json:"firstName"`
	LastName     string  `json:"lastName"`
	MemberType   *string `json:"memberType,omitempty"`
	UserKind     *string `json:"userKind,omitempty"`
	Language     *string `json:"language,omitempty"`
	FormatRegion *string `json:"formatRegion,omitempty"`
}

// patchMemberReq is the PATCH /v2/members/{memberId} body.
type patchMemberReq struct {
	Email        *string `json:"email,omitempty"`
	FirstName    *string `json:"firstName,omitempty"`
	LastName     *string `json:"lastName,omitempty"`
	MemberType   *string `json:"memberType,omitempty"`
	UserKind     *string `json:"userKind,omitempty"`
	Language     *string `json:"language,omitempty"`
	FormatRegion *string `json:"formatRegion,omitempty"`
}
