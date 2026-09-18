package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

// 变更签证审批结果状态。
var (
	// ErrAlreadyReviewed 变更单已被处理，重复或并发审批只有首个成功。
	ErrAlreadyReviewed = errors.New("change order already reviewed")
	// ErrPendingChangeExists 同一施工节点已存在待审批变更。
	ErrPendingChangeExists = errors.New("pending change order already exists for node")
)

// ChangeOrderRepository 施工变更签证仓储接口。
type ChangeOrderRepository interface {
	Create(order *model.ChangeOrder) error
	GetByID(id uint) (*model.ChangeOrder, error)
	List(filter ChangeOrderFilter, page, pageSize int) ([]model.ChangeOrder, int64, error)
	ListByProjectID(projectID uint) ([]model.ChangeOrder, error)
	// SubmitInTx 在同一事务内锁定施工节点并校验唯一性后创建待审批变更。
	SubmitInTx(order *model.ChangeOrder) error
	// ReviewInTx 在同一事务内完成审批：变更单、预算项、项目汇总一并生效或整单驳回。
	ReviewInTx(input ChangeReviewInput) (*model.ChangeOrder, error)
}

// ChangeOrderFilter 变更单查询过滤条件。
type ChangeOrderFilter struct {
	ProjectID uint
	NodeID    uint
	Status    string
}

// ChangeReviewInput 审批入参（事务内执行）。
type ChangeReviewInput struct {
	ChangeID   uint
	ReviewerID uint
	Approved   bool
	Comment    string
	Now        time.Time
}

type changeOrderRepository struct {
	db *gorm.DB
}

// NewChangeOrderRepository 构造变更签证仓储。
func NewChangeOrderRepository(db *gorm.DB) ChangeOrderRepository {
	return &changeOrderRepository{db: db}
}

func (r *changeOrderRepository) Create(order *model.ChangeOrder) error {
	if err := r.db.Create(order).Error; err != nil {
		return fmt.Errorf("create change order: %w", err)
	}
	return nil
}

func (r *changeOrderRepository) GetByID(id uint) (*model.ChangeOrder, error) {
	var order model.ChangeOrder
	if err := r.db.First(&order, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get change order %d: %w", id, err)
	}
	return &order, nil
}

func (r *changeOrderRepository) List(filter ChangeOrderFilter, page, pageSize int) ([]model.ChangeOrder, int64, error) {
	var orders []model.ChangeOrder
	var total int64
	query := r.db.Model(&model.ChangeOrder{})
	if filter.ProjectID != 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.NodeID != 0 {
		query = query.Where("node_id = ?", filter.NodeID)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count change orders: %w", err)
	}
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("list change orders: %w", err)
	}
	return orders, total, nil
}

func (r *changeOrderRepository) ListByProjectID(projectID uint) ([]model.ChangeOrder, error) {
	var orders []model.ChangeOrder
	if err := r.db.Where("project_id = ?", projectID).Order("id DESC").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("list change orders by project %d: %w", projectID, err)
	}
	return orders, nil
}

// forUpdate 在 MySQL 上追加行级锁；SQLite 等无锁方言自动忽略。
func forUpdate(tx *gorm.DB) *gorm.DB {
	if tx.Dialector.Name() == "mysql" {
		return tx.Clauses(clauseForUpdate())
	}
	return tx
}

func (r *changeOrderRepository) SubmitInTx(order *model.ChangeOrder) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// 锁定施工节点，串行化同一节点上的并发提交。
		var node model.ConstructionNode
		if err := forUpdate(tx).First(&node, order.NodeID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock construction node %d: %w", order.NodeID, err)
		}
		if node.ProjectID != order.ProjectID {
			return ErrNotFound
		}
		var count int64
		if err := tx.Model(&model.ChangeOrder{}).
			Where("node_id = ? AND status = ?", order.NodeID, model.StatusPendingChange).
			Count(&count).Error; err != nil {
			return fmt.Errorf("count pending change orders: %w", err)
		}
		if count > 0 {
			return ErrPendingChangeExists
		}
		if err := tx.Create(order).Error; err != nil {
			return fmt.Errorf("create change order: %w", err)
		}
		return nil
	})
}

func (r *changeOrderRepository) ReviewInTx(input ChangeReviewInput) (*model.ChangeOrder, error) {
	var result *model.ChangeOrder
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// 锁定项目行，串行化同项目并发审批，保证累计变更额校验一致。
		var order model.ChangeOrder
		if err := forUpdate(tx).First(&order, input.ChangeID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock change order %d: %w", input.ChangeID, err)
		}
		if order.Status != string(model.StatusPendingChange) {
			return ErrAlreadyReviewed
		}

		var project model.RenovationProject
		if err := forUpdate(tx).First(&project, order.ProjectID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return fmt.Errorf("lock project %d: %w", order.ProjectID, err)
		}

		// 驳回：变更单状态更新，预算与项目汇总保持不变。
		if !input.Approved {
			order.Status = string(model.StatusRejectedChange)
			order.ReviewerID = input.ReviewerID
			order.ReviewComment = input.Comment
			order.ReviewedAt = &input.Now
			if err := tx.Save(&order).Error; err != nil {
				return fmt.Errorf("reject change order: %w", err)
			}
			result = &order
			return nil
		}

		// 已用预算 = 项目下全部预算项实际花费累计；待审批变更不计入。
		used, err := sumActualAmount(tx, project.ID)
		if err != nil {
			return err
		}
		balance := project.ContractAmount - used
		if order.Amount > balance+changeAmountEpsilon {
			// 超出合同额与已用预算差额：整单拒绝，不改预算。
			order.Status = string(model.StatusRejectedChange)
			order.ReviewerID = input.ReviewerID
			order.ReviewComment = rejectCommentForOverBalance(input.Comment)
			order.ReviewedAt = &input.Now
			if err := tx.Save(&order).Error; err != nil {
				return fmt.Errorf("reject over-budget change order: %w", err)
			}
			result = &order
			return nil
		}

		// 批准：变更单、预算项、项目汇总在同一事务内一并生效。
		remark := fmt.Sprintf("施工变更签证 #%d（节点 #%d）增项", order.ID, order.NodeID)
		budget := &model.BudgetItem{
			ProjectID:    project.ID,
			Category:     changeBudgetCategory,
			BudgetAmount: round2(order.Amount),
			ActualAmount: round2(order.Amount),
			Variance:     0,
			Remark:       remark,
		}
		if err := tx.Create(budget).Error; err != nil {
			return fmt.Errorf("create change budget item: %w", err)
		}

		project.ApprovedChanges = round2(project.ApprovedChanges + order.Amount)
		project.ScheduleDelta += order.ScheduleImpact
		if project.ExpectedEndDate != nil && order.ScheduleImpact > 0 {
			newEnd := project.ExpectedEndDate.AddDate(0, 0, order.ScheduleImpact)
			project.ExpectedEndDate = &newEnd
		}
		if err := tx.Save(&project).Error; err != nil {
			return fmt.Errorf("update project change summary: %w", err)
		}

		order.Status = string(model.StatusApprovedChange)
		order.ReviewerID = input.ReviewerID
		order.ReviewComment = input.Comment
		order.ReviewedAt = &input.Now
		if err := tx.Save(&order).Error; err != nil {
			return fmt.Errorf("approve change order: %w", err)
		}
		result = &order
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
