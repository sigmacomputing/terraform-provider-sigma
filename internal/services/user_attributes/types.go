package user_attributes

import "time"

type defaultValue struct {
	Val  string `json:"val"`
	Type string `json:"type"` // always "string" per spec today
}

type userAttribute struct {
	UserAttributeID string        `json:"userAttributeId"`
	Name            string        `json:"name"`
	Description     *string       `json:"description"`
	DefaultValue    *defaultValue `json:"defaultValue"`
	CreatedBy       string        `json:"createdBy"`
	UpdatedBy       string        `json:"updatedBy"`
	CreatedAt       time.Time     `json:"createdAt"`
	UpdatedAt       time.Time     `json:"updatedAt"`
}

type createUARequest struct {
	Name         string        `json:"name"`
	Description  *string       `json:"description,omitempty"`
	DefaultValue *defaultValue `json:"defaultValue,omitempty"`
}

// userAssignment is one entry in a /users assignments POST or GET.
type userAssignment struct {
	UserID string       `json:"userId"`
	Value  defaultValue `json:"value"`
}

type teamAssignment struct {
	TeamID   string       `json:"teamId"`
	Value    defaultValue `json:"value"`
	Priority *float64     `json:"priority,omitempty"`
}

type assignmentsBody[T any] struct {
	Assignments []T `json:"assignments"`
}

type deleteUserAssignmentReq struct {
	Delete []struct {
		UserID string `json:"userId"`
	} `json:"delete"`
}

type deleteTeamAssignmentReq struct {
	Delete []struct {
		TeamID string `json:"teamId"`
	} `json:"delete"`
}
