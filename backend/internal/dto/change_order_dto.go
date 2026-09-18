package dto

import "time"

// CreateChangeOrderRequest 提交施工变更签证请求。
type CreateChangeOrderRequest struct {
	ProjectID      uint    `json:"project_id" binding:"required"`
	NodeID         uint    `json:"node_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"gt=0"`
	ScheduleImpact int     `json:"schedule_impact" binding:"gte=0"`
	Reason         string  `json:"reason" binding:"required,max=500"`
}

// ReviewChangeOrderRequest 审批施工变更签证请求。
type ReviewChangeOrderRequest struct {
	Approved bool   `json:"approved"`
	Comment  string `json:"comment" binding:"max=500"`
}

// ChangeOrderDTO 变更签证单展示结构。
type ChangeOrderDTO struct {
	ID             uint       `json:"id"`
	ProjectID      uint       `json:"project_id"`
	NodeID         uint       `json:"node_id"`
	Amount         float64    `json:"amount"`
	ScheduleImpact int        `json:"schedule_impact"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	ApplicantID    uint       `json:"applicant_id"`
	ReviewerID     uint       `json:"reviewer_id"`
	ReviewComment  string     `json:"review_comment"`
	ReviewedAt     *time.Time `json:"reviewed_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
