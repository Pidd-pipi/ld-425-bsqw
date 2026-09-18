package model

import "time"

// ChangeOrder 施工变更签证单。
type ChangeOrder struct {
	ID                 uint       `gorm:"primaryKey" json:"id"`
	ProjectID          uint       `gorm:"not null;index" json:"project_id"`
	NodeID             uint       `gorm:"not null;index" json:"node_id"`
	Title              string     `gorm:"size:120;not null" json:"title"`
	Amount             float64    `gorm:"type:decimal(14,2);not null" json:"amount"`
	ScheduleImpactDays int        `gorm:"not null;default:0" json:"schedule_impact_days"`
	Status             string     `gorm:"size:32;not null;index" json:"status"`
	SubmittedBy        uint       `gorm:"not null;index" json:"submitted_by"`
	ReviewedBy         uint       `gorm:"index" json:"reviewed_by"`
	ReviewNote         string     `gorm:"size:500" json:"review_note"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (ChangeOrder) TableName() string { return "change_orders" }
