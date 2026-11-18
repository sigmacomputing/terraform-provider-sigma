package workbooks

import "time"

type workbook struct {
	WorkbookID    string    `json:"workbookId"`
	WorkbookURLID string    `json:"workbookUrlId"`
	Name          string    `json:"name"`
	URL           string    `json:"url"`
	Path          string    `json:"path"`
	LatestVersion float64   `json:"latestVersion"`
	OwnerID       string    `json:"ownerId"`
	CreatedBy     string    `json:"createdBy"`
	UpdatedBy     string    `json:"updatedBy"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}
