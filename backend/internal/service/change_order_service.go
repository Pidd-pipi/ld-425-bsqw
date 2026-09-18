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
)

// ChangeOrderService 施工变更签证服务接口。
type ChangeOrderService interface {
	Submit(req *dto.CreateChangeOrderRequest, applicantID uint) (*model.ChangeOrder, error)
	GetByID(id uint) (*model.ChangeOrder, error)
	List(projectID, nodeID uint, status string, page, pageSize int) ([]model.ChangeOrder, int64, error)
	ListByProjectID(projectID uint) ([]model.ChangeOrder, error)
	Review(id uint, reviewerID uint, req *dto.ReviewChangeOrderRequest) (*model.ChangeOrder, error)
}

type changeOrderService struct {
	repo        repository.ChangeOrderRepository
	projectRepo repository.ProjectRepository
	nodeRepo    repository.ConstructionRepository
	logger      *slog.Logger
}

// NewChangeOrderService 构造施工变更签证服务。
func NewChangeOrderService(
	repo repository.ChangeOrderRepository,
	projectRepo repository.ProjectRepository,
	nodeRepo repository.ConstructionRepository,
	logger *slog.Logger,
) ChangeOrderService {
	return &changeOrderService{repo: repo, projectRepo: projectRepo, nodeRepo: nodeRepo, logger: logger}
}

func (s *changeOrderService) Submit(req *dto.CreateChangeOrderRequest, applicantID uint) (*model.ChangeOrder, error) {
	if _, err := s.projectRepo.GetByID(req.ProjectID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("project not found")
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	node, err := s.nodeRepo.GetByID(req.NodeID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("construction node not found")
		}
		return nil, fmt.Errorf("get construction node: %w", err)
	}
	if node.ProjectID != req.ProjectID {
		return nil, apperrors.NewBadRequest("construction node does not belong to project")
	}
	order := &model.ChangeOrder{
		ProjectID:      req.ProjectID,
		NodeID:         req.NodeID,
		Amount:         utils.Round2(req.Amount),
		ScheduleImpact: req.ScheduleImpact,
		Reason:         req.Reason,
		Status:         constants.ChangeOrderStatusPending,
		ApplicantID:    applicantID,
	}
	if err := s.repo.SubmitInTx(order); err != nil {
		if errors.Is(err, repository.ErrPendingChangeExists) {
			return nil, apperrors.NewConflict("该施工节点已存在待审批变更，同一节点只能有一个待审批变更")
		}
		return nil, fmt.Errorf("submit change order: %w", err)
	}
	s.logger.Info("change order submitted", "change_id", order.ID, "project_id", req.ProjectID, "node_id", req.NodeID, "applicant_id", applicantID)
	return order, nil
}

func (s *changeOrderService) GetByID(id uint) (*model.ChangeOrder, error) {
	order, err := s.repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, apperrors.NewNotFound("change order not found")
		}
		return nil, fmt.Errorf("get change order: %w", err)
	}
	return order, nil
}

func (s *changeOrderService) List(projectID, nodeID uint, status string, page, pageSize int) ([]model.ChangeOrder, int64, error) {
	orders, total, err := s.repo.List(repository.ChangeOrderFilter{ProjectID: projectID, NodeID: nodeID, Status: status}, page, pageSize)
	if err != nil {
		return nil, 0, fmt.Errorf("list change orders: %w", err)
	}
	return orders, total, nil
}

func (s *changeOrderService) ListByProjectID(projectID uint) ([]model.ChangeOrder, error) {
	orders, err := s.repo.ListByProjectID(projectID)
	if err != nil {
		return nil, fmt.Errorf("list change orders by project: %w", err)
	}
	return orders, nil
}

func (s *changeOrderService) Review(id uint, reviewerID uint, req *dto.ReviewChangeOrderRequest) (*model.ChangeOrder, error) {
	order, err := s.repo.ReviewInTx(repository.ChangeReviewInput{
		ChangeID:   id,
		ReviewerID: reviewerID,
		Approved:   req.Approved,
		Comment:    req.Comment,
		Now:        time.Now(),
	})
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrNotFound):
			return nil, apperrors.NewNotFound("change order not found")
		case errors.Is(err, repository.ErrAlreadyReviewed):
			return nil, apperrors.NewConflict("变更单已审批，重复或并发审批只允许首个成功")
		default:
			return nil, fmt.Errorf("review change order: %w", err)
		}
	}
	switch order.Status {
	case constants.ChangeOrderStatusApproved:
		s.logger.Info("change order approved", "change_id", id, "amount", order.Amount, "reviewer_id", reviewerID)
	case constants.ChangeOrderStatusRejected:
		s.logger.Info("change order rejected", "change_id", id, "reviewer_id", reviewerID, "comment", order.ReviewComment)
	}
	return order, nil
}
