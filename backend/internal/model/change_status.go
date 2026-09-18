package model

// ChangeStatus 变更签证单状态，取值与 constants.ChangeOrderStatus* 保持一致。
// 在 model 包定义常量以避免 repository 层反向依赖 constants 层。
type ChangeStatus string

const (
	StatusPendingChange  ChangeStatus = "Pending"
	StatusApprovedChange ChangeStatus = "Approved"
	StatusRejectedChange ChangeStatus = "Rejected"
)
