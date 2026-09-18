package model

import "time"

// ChangeOrder 施工变更签证单。
type ChangeOrder struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	ProjectID      uint       `gorm:"not null;index:idx_change_project_node,priority:1;index" json:"project_id"`
	NodeID         uint       `gorm:"not null;index:idx_change_project_node,priority:2;index" json:"node_id"`
	Amount         float64    `gorm:"type:decimal(14,2);not null;default:0" json:"amount"`
	ScheduleImpact int        `gorm:"not null;default:0" json:"schedule_impact"`
	Reason         string     `gorm:"size:500;not null" json:"reason"`
	Status         string     `gorm:"size:32;not null;index" json:"status"`
	ApplicantID    uint       `gorm:"not null" json:"applicant_id"`
	ReviewerID     uint       `json:"reviewer_id"`
	ReviewComment  string     `gorm:"size:500" json:"review_comment"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// TableName 指定表名。
func (ChangeOrder) TableName() string { return "change_orders" }
