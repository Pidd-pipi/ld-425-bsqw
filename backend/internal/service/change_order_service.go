package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"github.com/home-renovation/platform/internal/utils"
	"gorm.io/gorm"
)

// ChangeOrderService 施工变更签证服务接口。
type ChangeOrderService interface {
	Create(submitterID uint, req *dto.CreateChangeOrderRequest) (*model.ChangeOrder, error)
	GetByID(id uint) (*model.ChangeOrder, error)
	List(filter repository.ChangeOrderFilter, page, pageSize int) ([]model.ChangeOrder, int64, error)
	ListByProjectID(projectID uint, status string) ([]model.ChangeOrder, error)
	Review(id uint, reviewerID uint, req *dto.ReviewChangeOrderRequest) (*model.ChangeOrder, error)
	Summary(projectID uint) (*dto.ChangeOrderSummaryDTO, error)
}

type changeOrderService struct {
	changeRepo       repository.ChangeOrderRepository
	projectRepo      repository.ProjectRepository
	budgetRepo       repository.BudgetRepository
	constructionRepo repository.ConstructionRepository
	txManager        repository.TransactionManager
	logger           *slog.Logger
}

// NewChangeOrderService 构造变更签证服务。
func NewChangeOrderService(
	changeRepo repository.ChangeOrderRepository,
	projectRepo repository.ProjectRepository,
	budgetRepo repository.BudgetRepository,
	constructionRepo repository.ConstructionRepository,
	txManager repository.TransactionManager,
	logger *slog.Logger,
) ChangeOrderService {
	return &changeOrderService{
		changeRepo:       changeRepo,
		projectRepo:      projectRepo,
		budgetRepo:       budgetRepo,
		constructionRepo: constructionRepo,
		txManager:        txManager,
		logger:           logger,
	}
}

// Create 施工方按项目与施工节点提交变更签证，待审批不计入已用预算。
func (s *changeOrderService) Create(submitterID uint, req *dto.CreateChangeOrderRequest) (*model.ChangeOrder, error) {
	if _, err := s.getProject(req.ProjectID); err != nil {
		return nil, err
	}
	node, err := s.constructionRepo.GetByID(req.NodeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("construction node not found")
		}
		return nil, fmt.Errorf("get construction node: %w", err)
	}
	if node.ProjectID != req.ProjectID {
		return nil, apperrors.NewBadRequest("construction node does not belong to the project")
	}
	hasPending, err := s.changeRepo.HasPendingByNodeID(req.NodeID)
	if err != nil {
		return nil, fmt.Errorf("check pending change order: %w", err)
	}
	if hasPending {
		return nil, apperrors.NewConflict("该节点已存在待审批的变更单")
	}
	order := &model.ChangeOrder{
		ProjectID:          req.ProjectID,
		NodeID:             req.NodeID,
		Title:              req.Title,
		Amount:             utils.Round2(req.Amount),
		ScheduleImpactDays: req.ScheduleImpactDays,
		Status:             constants.ChangeOrderStatusPending,
		SubmittedBy:        submitterID,
	}
	if err := s.changeRepo.Create(order); err != nil {
		if errors.Is(err, repository.ErrDuplicatePending) {
			return nil, apperrors.NewConflict("该节点已存在待审批的变更单")
		}
		return nil, fmt.Errorf("create change order: %w", err)
	}
	s.logger.Info("change order submitted", "order_id", order.ID, "project_id", order.ProjectID, "node_id", order.NodeID, "amount", order.Amount)
	return order, nil
}

func (s *changeOrderService) GetByID(id uint) (*model.ChangeOrder, error) {
	order, err := s.changeRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("change order not found")
		}
		return nil, fmt.Errorf("get change order: %w", err)
	}
	return order, nil
}

func (s *changeOrderService) List(filter repository.ChangeOrderFilter, page, pageSize int) ([]model.ChangeOrder, int64, error) {
	orders, total, err := s.changeRepo.List(filter, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list change orders: %w", err)
	}
	return orders, total, nil
}

func (s *changeOrderService) ListByProjectID(projectID uint, status string) ([]model.ChangeOrder, error) {
	orders, err := s.changeRepo.ListByProjectID(projectID, status)
	if err != nil {
		return nil, fmt.Errorf("list change orders by project: %w", err)
	}
	return orders, nil
}

// Review 项目经理或业主审批变更签证。
// 批准时校验累计变更额不得超过合同额与已用预算的差额，超出则整单拒绝；
// 变更单、预算项与项目汇总在同一事务生效；驳回不改预算；
// 重复或并发审批通过条件更新保证仅首个成功。
func (s *changeOrderService) Review(id uint, reviewerID uint, req *dto.ReviewChangeOrderRequest) (*model.ChangeOrder, error) {
	targetStatus := constants.ChangeOrderStatusRejected
	if req.Approved {
		targetStatus = constants.ChangeOrderStatusApproved
	}
	now := time.Now()
	err := s.txManager.InTransaction(func(tx *gorm.DB) error {
		changeRepo := s.changeRepo.WithTx(tx)
		order, err := changeRepo.GetByID(id)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return apperrors.NewNotFound("change order not found")
			}
			return fmt.Errorf("get change order: %w", err)
		}
		if order.Status != constants.ChangeOrderStatusPending {
			return apperrors.NewConflict("变更单已审批，请勿重复操作")
		}
		marked, err := changeRepo.MarkReviewed(id, targetStatus, reviewerID, req.Note, now)
		if err != nil {
			return fmt.Errorf("review change order: %w", err)
		}
		if !marked {
			return apperrors.NewConflict("变更单已审批，请勿重复操作")
		}
		if req.Approved {
			if err := s.applyApproval(tx, order); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	order, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	s.logger.Info("change order reviewed", "order_id", id, "status", targetStatus, "reviewer_id", reviewerID)
	return order, nil
}

// applyApproval 在事务内完成预算校验、预算项落账与项目汇总更新。
func (s *changeOrderService) applyApproval(tx *gorm.DB, order *model.ChangeOrder) error {
	projectRepo := s.projectRepo.WithTx(tx)
	budgetRepo := s.budgetRepo.WithTx(tx)
	project, err := projectRepo.GetByIDForUpdate(order.ProjectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return apperrors.NewNotFound("project not found")
		}
		return fmt.Errorf("lock project: %w", err)
	}
	usedBudget, err := budgetRepo.SumActualByProjectIDForUpdate(order.ProjectID)
	if err != nil {
		return fmt.Errorf("sum used budget: %w", err)
	}
	if utils.Round2(usedBudget+order.Amount) > utils.Round2(project.ContractAmount) {
		return apperrors.NewConflict("累计变更额超出合同额与已用预算的差额，整单拒绝")
	}
	item := &model.BudgetItem{
		ProjectID:    order.ProjectID,
		Category:     "Other",
		BudgetAmount: order.Amount,
		ActualAmount: order.Amount,
		Remark:       fmt.Sprintf("施工变更签证#%d %s", order.ID, order.Title),
	}
	item.Variance = utils.BudgetVariance(item.BudgetAmount, item.ActualAmount)
	if err := budgetRepo.Create(item); err != nil {
		return fmt.Errorf("create change budget item: %w", err)
	}
	project.ApprovedChangeAmount = utils.Round2(project.ApprovedChangeAmount + order.Amount)
	if err := projectRepo.Update(project); err != nil {
		return fmt.Errorf("update project change summary: %w", err)
	}
	return nil
}

// Summary 项目变更汇总：待审批与已批准金额、已用预算与剩余可变更额度。
func (s *changeOrderService) Summary(projectID uint) (*dto.ChangeOrderSummaryDTO, error) {
	project, err := s.getProject(projectID)
	if err != nil {
		return nil, err
	}
	usedBudget, err := s.budgetRepo.SumActualByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("sum used budget: %w", err)
	}
	pendingAmount, err := s.changeRepo.SumAmountByProjectAndStatus(projectID, constants.ChangeOrderStatusPending)
	if err != nil {
		return nil, err
	}
	approvedAmount, err := s.changeRepo.SumAmountByProjectAndStatus(projectID, constants.ChangeOrderStatusApproved)
	if err != nil {
		return nil, err
	}
	pendingCount, err := s.changeRepo.CountByProjectAndStatus(projectID, constants.ChangeOrderStatusPending)
	if err != nil {
		return nil, err
	}
	approvedCount, err := s.changeRepo.CountByProjectAndStatus(projectID, constants.ChangeOrderStatusApproved)
	if err != nil {
		return nil, err
	}
	return &dto.ChangeOrderSummaryDTO{
		ProjectID:       projectID,
		ContractAmount:  project.ContractAmount,
		UsedBudget:      utils.Round2(usedBudget),
		PendingAmount:   utils.Round2(pendingAmount),
		ApprovedAmount:  utils.Round2(approvedAmount),
		PendingCount:    pendingCount,
		ApprovedCount:   approvedCount,
		RemainingAmount: utils.Round2(project.ContractAmount - usedBudget),
	}, nil
}

func (s *changeOrderService) getProject(projectID uint) (*model.RenovationProject, error) {
	project, err := s.projectRepo.GetByID(projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("project not found")
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return project, nil
}
