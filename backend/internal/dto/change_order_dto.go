package dto

import "time"

// CreateChangeOrderRequest 提交施工变更签证请求。
type CreateChangeOrderRequest struct {
	ProjectID          uint    `json:"project_id" binding:"required"`
	NodeID             uint    `json:"node_id" binding:"required"`
	Title              string  `json:"title" binding:"required,max=120"`
	Amount             float64 `json:"amount" binding:"required,gt=0"`
	ScheduleImpactDays int     `json:"schedule_impact_days" binding:"gte=0"`
}

// ReviewChangeOrderRequest 审批变更签证请求。
type ReviewChangeOrderRequest struct {
	Approved bool   `json:"approved"`
	Note     string `json:"note" binding:"max=500"`
}

// ChangeOrderDTO 变更签证展示结构。
type ChangeOrderDTO struct {
	ID                 uint       `json:"id"`
	ProjectID          uint       `json:"project_id"`
	NodeID             uint       `json:"node_id"`
	Title              string     `json:"title"`
	Amount             float64    `json:"amount"`
	ScheduleImpactDays int        `json:"schedule_impact_days"`
	Status             string     `json:"status"`
	SubmittedBy        uint       `json:"submitted_by"`
	ReviewedBy         uint       `json:"reviewed_by"`
	ReviewNote         string     `json:"review_note"`
	ReviewedAt         *time.Time `json:"reviewed_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// ChangeOrderSummaryDTO 项目变更汇总：待审批与已批准金额及预算余量。
type ChangeOrderSummaryDTO struct {
	ProjectID       uint    `json:"project_id"`
	ContractAmount  float64 `json:"contract_amount"`
	UsedBudget      float64 `json:"used_budget"`
	PendingAmount   float64 `json:"pending_amount"`
	ApprovedAmount  float64 `json:"approved_amount"`
	PendingCount    int64   `json:"pending_count"`
	ApprovedCount   int64   `json:"approved_count"`
	RemainingAmount float64 `json:"remaining_amount"`
}
