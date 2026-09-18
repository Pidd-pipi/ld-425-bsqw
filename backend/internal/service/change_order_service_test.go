package service

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	apperrors "github.com/home-renovation/platform/internal/errors"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
)

type fakeChangeOrderRepo struct {
	nextID    uint
	items     map[uint]*model.ChangeOrder
	submitErr error
}

func newFakeChangeOrderRepo() *fakeChangeOrderRepo {
	return &fakeChangeOrderRepo{nextID: 1, items: map[uint]*model.ChangeOrder{}}
}

func (f *fakeChangeOrderRepo) Create(order *model.ChangeOrder) error {
	order.ID = f.nextID
	f.nextID++
	f.items[order.ID] = order
	return nil
}

func (f *fakeChangeOrderRepo) GetByID(id uint) (*model.ChangeOrder, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return item, nil
}

func (f *fakeChangeOrderRepo) List(filter repository.ChangeOrderFilter, page, pageSize int) ([]model.ChangeOrder, int64, error) {
	var out []model.ChangeOrder
	for _, item := range f.items {
		if filter.ProjectID != 0 && item.ProjectID != filter.ProjectID {
			continue
		}
		if filter.NodeID != 0 && item.NodeID != filter.NodeID {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		out = append(out, *item)
	}
	return out, int64(len(out)), nil
}

func (f *fakeChangeOrderRepo) ListByProjectID(projectID uint) ([]model.ChangeOrder, error) {
	var out []model.ChangeOrder
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeChangeOrderRepo) SubmitInTx(order *model.ChangeOrder) error {
	if f.submitErr != nil {
		return f.submitErr
	}
	return f.Create(order)
}

func (f *fakeChangeOrderRepo) ReviewInTx(input repository.ChangeReviewInput) (*model.ChangeOrder, error) {
	item, ok := f.items[input.ChangeID]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if item.Status != constants.ChangeOrderStatusPending {
		return nil, repository.ErrAlreadyReviewed
	}
	now := input.Now
	item.ReviewerID = input.ReviewerID
	item.ReviewComment = input.Comment
	item.ReviewedAt = &now
	if input.Approved {
		item.Status = constants.ChangeOrderStatusApproved
	} else {
		item.Status = constants.ChangeOrderStatusRejected
	}
	return item, nil
}

type fakeProjectRepoForChange struct {
	projects map[uint]*model.RenovationProject
}

func (f *fakeProjectRepoForChange) Create(project *model.RenovationProject) error {
	f.projects[project.ID] = project
	return nil
}
func (f *fakeProjectRepoForChange) GetByID(id uint) (*model.RenovationProject, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}
func (f *fakeProjectRepoForChange) List(_ repository.ProjectFilter, _, _ int) ([]model.RenovationProject, int64, error) {
	return nil, 0, nil
}
func (f *fakeProjectRepoForChange) ListAll() ([]model.RenovationProject, error) { return nil, nil }
func (f *fakeProjectRepoForChange) Update(project *model.RenovationProject) error {
	f.projects[project.ID] = project
	return nil
}
func (f *fakeProjectRepoForChange) Delete(id uint) error {
	delete(f.projects, id)
	return nil
}

type fakeNodeRepoForChange struct {
	nodes map[uint]*model.ConstructionNode
}

func (f *fakeNodeRepoForChange) Create(node *model.ConstructionNode) error {
	f.nodes[node.ID] = node
	return nil
}
func (f *fakeNodeRepoForChange) GetByID(id uint) (*model.ConstructionNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return n, nil
}
func (f *fakeNodeRepoForChange) List(_ repository.ConstructionFilter, _, _ int) ([]model.ConstructionNode, int64, error) {
	return nil, 0, nil
}
func (f *fakeNodeRepoForChange) ListByProjectID(projectID uint) ([]model.ConstructionNode, error) {
	var out []model.ConstructionNode
	for _, n := range f.nodes {
		if n.ProjectID == projectID {
			out = append(out, *n)
		}
	}
	return out, nil
}
func (f *fakeNodeRepoForChange) Update(node *model.ConstructionNode) error {
	f.nodes[node.ID] = node
	return nil
}
func (f *fakeNodeRepoForChange) Delete(id uint) error {
	delete(f.nodes, id)
	return nil
}

func newChangeServiceWithFakes() (*changeOrderService, *fakeChangeOrderRepo) {
	projectRepo := &fakeProjectRepoForChange{projects: map[uint]*model.RenovationProject{
		1: {ID: 1, Name: "项目", Status: constants.ProjectStatusInProgress, ContractAmount: 100000},
	}}
	nodeRepo := &fakeNodeRepoForChange{nodes: map[uint]*model.ConstructionNode{
		10: {ID: 10, ProjectID: 1, Name: "水电", Status: constants.ConstructionStatusInProgress, AcceptanceStatus: constants.AcceptanceStatusPending},
		20: {ID: 20, ProjectID: 2, Name: "木工", Status: constants.ConstructionStatusPending, AcceptanceStatus: constants.AcceptanceStatusPending},
	}}
	repo := newFakeChangeOrderRepo()
	svc := NewChangeOrderService(repo, projectRepo, nodeRepo, slog.Default())
	return svc.(*changeOrderService), repo
}

func TestChangeOrderService_Submit(t *testing.T) {
	svc, repo := newChangeServiceWithFakes()
	order, err := svc.Submit(&dto.CreateChangeOrderRequest{
		ProjectID: 1, NodeID: 10, Amount: 5000, ScheduleImpact: 2, Reason: "新增管线",
	}, 3)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if order.Status != constants.ChangeOrderStatusPending || order.ApplicantID != 3 {
		t.Fatalf("unexpected order: %+v", order)
	}
	if repo.items[order.ID].Amount != 5000 {
		t.Fatalf("expected amount persisted, got %v", repo.items[order.ID].Amount)
	}
}

func TestChangeOrderService_SubmitNodeMismatch(t *testing.T) {
	svc, _ := newChangeServiceWithFakes()
	_, err := svc.Submit(&dto.CreateChangeOrderRequest{
		ProjectID: 1, NodeID: 20, Amount: 1000, Reason: "跨项目节点",
	}, 3)
	var businessErr *apperrors.BusinessError
	if !errors.As(err, &businessErr) || businessErr.Code != constants.CodeBadRequest {
		t.Fatalf("expected bad request for mismatched node, got %v", err)
	}
}

func TestChangeOrderService_SubmitProjectNotFound(t *testing.T) {
	svc, _ := newChangeServiceWithFakes()
	_, err := svc.Submit(&dto.CreateChangeOrderRequest{
		ProjectID: 99, NodeID: 10, Amount: 1000, Reason: "不存在项目",
	}, 3)
	var businessErr *apperrors.BusinessError
	if !errors.As(err, &businessErr) || businessErr.HTTP != 404 {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestChangeOrderService_SubmitPendingConflict(t *testing.T) {
	svc, repo := newChangeServiceWithFakes()
	repo.submitErr = repository.ErrPendingChangeExists
	_, err := svc.Submit(&dto.CreateChangeOrderRequest{
		ProjectID: 1, NodeID: 10, Amount: 1000, Reason: "重复待审批",
	}, 3)
	var businessErr *apperrors.BusinessError
	if !errors.As(err, &businessErr) || businessErr.HTTP != 409 {
		t.Fatalf("expected conflict, got %v", err)
	}
}

func TestChangeOrderService_ReviewApproveAndReject(t *testing.T) {
	tests := []struct {
		name     string
		approved bool
		want     string
	}{
		{name: "approve", approved: true, want: constants.ChangeOrderStatusApproved},
		{name: "reject", approved: false, want: constants.ChangeOrderStatusRejected},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newChangeServiceWithFakes()
			order, err := svc.Submit(&dto.CreateChangeOrderRequest{ProjectID: 1, NodeID: 10, Amount: 1000, Reason: "x"}, 3)
			if err != nil {
				t.Fatalf("submit: %v", err)
			}
			got, err := svc.Review(order.ID, 5, &dto.ReviewChangeOrderRequest{Approved: tt.approved, Comment: "意见"})
			if err != nil {
				t.Fatalf("review: %v", err)
			}
			if got.Status != tt.want || got.ReviewerID != 5 || got.ReviewedAt == nil {
				t.Fatalf("unexpected reviewed order: %+v", got)
			}
		})
	}
}

func TestChangeOrderService_ReviewDuplicateConflict(t *testing.T) {
	svc, _ := newChangeServiceWithFakes()
	order, err := svc.Submit(&dto.CreateChangeOrderRequest{ProjectID: 1, NodeID: 10, Amount: 1000, Reason: "x"}, 3)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if _, err := svc.Review(order.ID, 5, &dto.ReviewChangeOrderRequest{Approved: true}); err != nil {
		t.Fatalf("first review: %v", err)
	}
	_, err = svc.Review(order.ID, 5, &dto.ReviewChangeOrderRequest{Approved: true})
	var businessErr *apperrors.BusinessError
	if !errors.As(err, &businessErr) || businessErr.HTTP != 409 {
		t.Fatalf("expected conflict on duplicate review, got %v", err)
	}
}

func TestChangeOrderService_ListByStatus(t *testing.T) {
	svc, _ := newChangeServiceWithFakes()
	if _, err := svc.Submit(&dto.CreateChangeOrderRequest{ProjectID: 1, NodeID: 10, Amount: 1000, Reason: "pending"}, 3); err != nil {
		t.Fatalf("submit: %v", err)
	}
	orders, total, err := svc.List(1, 10, constants.ChangeOrderStatusPending, 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(orders) != 1 || orders[0].Status != constants.ChangeOrderStatusPending {
		t.Fatalf("expected one pending order, got total=%d orders=%+v", total, orders)
	}
	none, noneTotal, err := svc.List(1, 10, constants.ChangeOrderStatusApproved, 1, 20)
	if err != nil {
		t.Fatalf("list approved: %v", err)
	}
	if noneTotal != 0 || len(none) != 0 {
		t.Fatalf("expected no approved orders, got %+v", none)
	}
}

func TestChangeOrderService_PendingNotCountedInList(t *testing.T) {
	svc, repo := newChangeServiceWithFakes()
	if _, err := svc.Submit(&dto.CreateChangeOrderRequest{ProjectID: 1, NodeID: 10, Amount: 1000, Reason: "pending"}, 3); err != nil {
		t.Fatalf("submit: %v", err)
	}
	orders, err := svc.ListByProjectID(1)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(orders) != 1 || orders[0].Status != constants.ChangeOrderStatusPending {
		t.Fatalf("expected one pending order, got %+v", orders)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 stored order, got %d", len(repo.items))
	}
}
