package teams

import "time"

type team struct {
	TeamID      string    `json:"teamId"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Visibility  string    `json:"visibility"`
	CreatedBy   string    `json:"createdBy"`
	UpdatedBy   string    `json:"updatedBy"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsArchived  *bool     `json:"isArchived,omitempty"`
}

type createTeamReq struct {
	Name             string  `json:"name"`
	Description      *string `json:"description,omitempty"`
	Visibility       *string `json:"visibility,omitempty"`
	CreateTeamFolder *bool   `json:"createTeamFolder,omitempty"`
}

type patchTeamReq struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Visibility  *string `json:"visibility,omitempty"`
}

type teamMembersPatchReq struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

type teamMember struct {
	UserID      string `json:"userId"`
	IsTeamAdmin bool   `json:"isTeamAdmin"`
	AddedBy     string `json:"addedBy"`
}
