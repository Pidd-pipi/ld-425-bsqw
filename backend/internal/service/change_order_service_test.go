package service

import (
	"log/slog"
	"testing"
	"time"

	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/dto"
	"github.com/home-renovation/platform/internal/model"
	"github.com/home-renovation/platform/internal/repository"
	"gorm.io/gorm"
)

// fakeChangeOrderRepo 测试用内存变更签证仓储。
type fakeChangeOrderRepo struct {
	nextID uint
	items  map[uint]*model.ChangeOrder
}

func newFakeChangeOrderRepo() *fakeChangeOrderRepo {
	return &fakeChangeOrderRepo{nextID: 1, items: map[uint]*model.ChangeOrder{}}
}

func (f *fakeChangeOrderRepo) Create(order *model.ChangeOrder) error {
	for _, item := range f.items {
		if item.NodeID == order.NodeID && item.Status == constants.ChangeOrderStatusPending {
			return repository.ErrDuplicatePending
		}
	}
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

func (f *fakeChangeOrderRepo) ListByProjectID(projectID uint, status string) ([]model.ChangeOrder, error) {
	var out []model.ChangeOrder
	for _, item := range f.items {
		if item.ProjectID != projectID {
			continue
		}
		if status != "" && item.Status != status {
			continue
		}
		out = append(out, *item)
	}
	return out, nil
}

func (f *fakeChangeOrderRepo) HasPendingByNodeID(nodeID uint) (bool, error) {
	for _, item := range f.items {
		if item.NodeID == nodeID && item.Status == constants.ChangeOrderStatusPending {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeChangeOrderRepo) SumAmountByProjectAndStatus(projectID uint, status string) (float64, error) {
	var total float64
	for _, item := range f.items {
		if item.ProjectID == projectID && item.Status == status {
			total += item.Amount
		}
	}
	return total, nil
}

func (f *fakeChangeOrderRepo) CountByProjectAndStatus(projectID uint, status string) (int64, error) {
	var count int64
	for _, item := range f.items {
		if item.ProjectID == projectID && item.Status == status {
			count++
		}
	}
	return count, nil
}

func (f *fakeChangeOrderRepo) MarkReviewed(id uint, status string, reviewedBy uint, note string, reviewedAt time.Time) (bool, error) {
	item, ok := f.items[id]
	if !ok || item.Status != constants.ChangeOrderStatusPending {
		return false, nil
	}
	item.Status = status
	item.ReviewedBy = reviewedBy
	item.ReviewNote = note
	item.ReviewedAt = &reviewedAt
	return true, nil
}

func (f *fakeChangeOrderRepo) WithTx(tx *gorm.DB) repository.ChangeOrderRepository { return f }

// snapshot 深拷贝当前数据，用于模拟事务回滚。
func (f *fakeChangeOrderRepo) snapshot() map[uint]model.ChangeOrder {
	snap := make(map[uint]model.ChangeOrder, len(f.items))
	for id, item := range f.items {
		snap[id] = *item
	}
	return snap
}

func (f *fakeChangeOrderRepo) restore(snap map[uint]model.ChangeOrder) {
	f.items = make(map[uint]*model.ChangeOrder, len(snap))
	for id, item := range snap {
		copied := item
		f.items[id] = &copied
	}
}

// fakeProjectRepo 测试用内存项目仓储。
type fakeProjectRepo struct {
	items map[uint]*model.RenovationProject
}

func newFakeProjectRepo() *fakeProjectRepo {
	return &fakeProjectRepo{items: map[uint]*model.RenovationProject{}}
}

func (f *fakeProjectRepo) Create(project *model.RenovationProject) error {
	project.ID = uint(len(f.items) + 1)
	f.items[project.ID] = project
	return nil
}

func (f *fakeProjectRepo) GetByID(id uint) (*model.RenovationProject, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return item, nil
}

func (f *fakeProjectRepo) GetByIDForUpdate(id uint) (*model.RenovationProject, error) {
	return f.GetByID(id)
}

func (f *fakeProjectRepo) List(filter repository.ProjectFilter, page, pageSize int) ([]model.RenovationProject, int64, error) {
	return nil, 0, nil
}

func (f *fakeProjectRepo) ListAll() ([]model.RenovationProject, error) { return nil, nil }

func (f *fakeProjectRepo) Update(project *model.RenovationProject) error {
	if _, ok := f.items[project.ID]; !ok {
		return repository.ErrNotFound
	}
	f.items[project.ID] = project
	return nil
}

func (f *fakeProjectRepo) Delete(id uint) error { return nil }

func (f *fakeProjectRepo) WithTx(tx *gorm.DB) repository.ProjectRepository { return f }

func (f *fakeProjectRepo) snapshot() map[uint]model.RenovationProject {
	snap := make(map[uint]model.RenovationProject, len(f.items))
	for id, item := range f.items {
		snap[id] = *item
	}
	return snap
}

func (f *fakeProjectRepo) restore(snap map[uint]model.RenovationProject) {
	f.items = make(map[uint]*model.RenovationProject, len(snap))
	for id, item := range snap {
		copied := item
		f.items[id] = &copied
	}
}

// fakeBudgetRepo 测试用内存预算仓储。
type fakeBudgetRepo struct {
	nextID uint
	items  map[uint]*model.BudgetItem
}

func newFakeBudgetRepo() *fakeBudgetRepo {
	return &fakeBudgetRepo{nextID: 1, items: map[uint]*model.BudgetItem{}}
}

func (f *fakeBudgetRepo) Create(item *model.BudgetItem) error {
	item.ID = f.nextID
	f.nextID++
	f.items[item.ID] = item
	return nil
}

func (f *fakeBudgetRepo) GetByID(id uint) (*model.BudgetItem, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return item, nil
}

func (f *fakeBudgetRepo) List(filter repository.BudgetFilter, page, pageSize int) ([]model.BudgetItem, int64, error) {
	return nil, 0, nil
}

func (f *fakeBudgetRepo) ListByProjectID(projectID uint) ([]model.BudgetItem, error) {
	var out []model.BudgetItem
	for _, item := range f.items {
		if item.ProjectID == projectID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeBudgetRepo) SumActualByProjectID(projectID uint) (float64, error) {
	var total float64
	for _, item := range f.items {
		if item.ProjectID == projectID {
			total += item.ActualAmount
		}
	}
	return total, nil
}

func (f *fakeBudgetRepo) SumActualByProjectIDForUpdate(projectID uint) (float64, error) {
	return f.SumActualByProjectID(projectID)
}

func (f *fakeBudgetRepo) Update(item *model.BudgetItem) error { return nil }

func (f *fakeBudgetRepo) Delete(id uint) error { return nil }

func (f *fakeBudgetRepo) WithTx(tx *gorm.DB) repository.BudgetRepository { return f }

func (f *fakeBudgetRepo) snapshot() map[uint]model.BudgetItem {
	snap := make(map[uint]model.BudgetItem, len(f.items))
	for id, item := range f.items {
		snap[id] = *item
	}
	return snap
}

func (f *fakeBudgetRepo) restore(snap map[uint]model.BudgetItem) {
	f.items = make(map[uint]*model.BudgetItem, len(snap))
	for id, item := range snap {
		copied := item
		f.items[id] = &copied
	}
}

// fakeConstructionRepo 测试用内存施工节点仓储。
type fakeConstructionRepo struct {
	items map[uint]*model.ConstructionNode
}

func newFakeConstructionRepo() *fakeConstructionRepo {
	return &fakeConstructionRepo{items: map[uint]*model.ConstructionNode{}}
}

func (f *fakeConstructionRepo) Create(node *model.ConstructionNode) error {
	node.ID = uint(len(f.items) + 1)
	f.items[node.ID] = node
	return nil
}

func (f *fakeConstructionRepo) GetByID(id uint) (*model.ConstructionNode, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return item, nil
}

func (f *fakeConstructionRepo) List(filter repository.ConstructionFilter, page, pageSize int) ([]model.ConstructionNode, int64, error) {
	return nil, 0, nil
}

func (f *fakeConstructionRepo) ListByProjectID(projectID uint) ([]model.ConstructionNode, error) {
	return nil, nil
}

func (f *fakeConstructionRepo) Update(node *model.ConstructionNode) error { return nil }

func (f *fakeConstructionRepo) Delete(id uint) error { return nil }

// fakeTxManager 直接执行事务函数，出错时回滚各仓储数据。
type fakeTxManager struct {
	changes  *fakeChangeOrderRepo
	projects *fakeProjectRepo
	budgets  *fakeBudgetRepo
}

func (m *fakeTxManager) InTransaction(fn func(tx *gorm.DB) error) error {
	changeSnap := m.changes.snapshot()
	projectSnap := m.projects.snapshot()
	budgetSnap := m.budgets.snapshot()
	if err := fn(nil); err != nil {
		m.changes.restore(changeSnap)
		m.projects.restore(projectSnap)
		m.budgets.restore(budgetSnap)
		return err
	}
	return nil
}

type changeOrderTestFixture struct {
	svc          ChangeOrderService
	changes      *fakeChangeOrderRepo
	projects     *fakeProjectRepo
	budgets      *fakeBudgetRepo
	construction *fakeConstructionRepo
}

func newChangeOrderFixture() *changeOrderTestFixture {
	changes := newFakeChangeOrderRepo()
	projects := newFakeProjectRepo()
	budgets := newFakeBudgetRepo()
	construction := newFakeConstructionRepo()
	txManager := &fakeTxManager{changes: changes, projects: projects, budgets: budgets}
	svc := NewChangeOrderService(changes, projects, budgets, construction, txManager, slog.Default())
	return &changeOrderTestFixture{svc: svc, changes: changes, projects: projects, budgets: budgets, construction: construction}
}

func (f *changeOrderTestFixture) seedProject(contract float64, usedBudget float64) *model.RenovationProject {
	project := &model.RenovationProject{Name: "测试项目", ContractAmount: contract, Status: constants.ProjectStatusInProgress}
	_ = f.projects.Create(project)
	if usedBudget > 0 {
		_ = f.budgets.Create(&model.BudgetItem{ProjectID: project.ID, Category: "Labor", BudgetAmount: usedBudget, ActualAmount: usedBudget})
	}
	return project
}

func (f *changeOrderTestFixture) seedNode(projectID uint) *model.ConstructionNode {
	node := &model.ConstructionNode{ProjectID: projectID, Name: "水电", Status: constants.ConstructionStatusInProgress}
	_ = f.construction.Create(node)
	return node
}

func TestChangeOrderService_CreatePending(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 40000)
	node := fx.seedNode(project.ID)

	order, err := fx.svc.Create(7, &dto.CreateChangeOrderRequest{
		ProjectID: project.ID, NodeID: node.ID, Title: "增加插座", Amount: 5000, ScheduleImpactDays: 3,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if order.Status != constants.ChangeOrderStatusPending {
		t.Fatalf("expected pending status, got %q", order.Status)
	}

	// 待审批不计入已用预算。
	summary, err := fx.svc.Summary(project.ID)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.UsedBudget != 40000 {
		t.Fatalf("expected used budget 40000, got %v", summary.UsedBudget)
	}
	if summary.PendingAmount != 5000 || summary.PendingCount != 1 {
		t.Fatalf("expected pending 5000/1, got %v/%d", summary.PendingAmount, summary.PendingCount)
	}
	if summary.ApprovedAmount != 0 {
		t.Fatalf("expected approved 0, got %v", summary.ApprovedAmount)
	}
}

func TestChangeOrderService_CreateDuplicatePendingConflict(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 0)
	node := fx.seedNode(project.ID)
	req := &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "增项", Amount: 1000}

	if _, err := fx.svc.Create(7, req); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := fx.svc.Create(7, req); err == nil {
		t.Fatal("expected conflict for second pending order on same node")
	}

	// 其他节点仍可提交。
	other := fx.seedNode(project.ID)
	if _, err := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: other.ID, Title: "增项2", Amount: 2000}); err != nil {
		t.Fatalf("create on other node: %v", err)
	}
}

func TestChangeOrderService_CreateNodeMismatch(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 0)
	otherProject := fx.seedProject(80000, 0)
	node := fx.seedNode(otherProject.ID)

	if _, err := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "增项", Amount: 1000}); err == nil {
		t.Fatal("expected error when node does not belong to project")
	}
}

func TestChangeOrderService_ApproveAppliesBudgetAndSummary(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 40000)
	node := fx.seedNode(project.ID)
	order, err := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "增项", Amount: 10000, ScheduleImpactDays: 5})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := fx.svc.Review(order.ID, 9, &dto.ReviewChangeOrderRequest{Approved: true, Note: "同意"})
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got.Status != constants.ChangeOrderStatusApproved {
		t.Fatalf("expected approved, got %q", got.Status)
	}
	if got.ReviewedBy != 9 || got.ReviewedAt == nil {
		t.Fatalf("expected reviewer recorded, got %+v", got)
	}

	// 预算项与项目汇总一并生效。
	used, _ := fx.budgets.SumActualByProjectID(project.ID)
	if used != 50000 {
		t.Fatalf("expected used budget 50000 after approval, got %v", used)
	}
	updated, _ := fx.projects.GetByID(project.ID)
	if updated.ApprovedChangeAmount != 10000 {
		t.Fatalf("expected project change summary 10000, got %v", updated.ApprovedChangeAmount)
	}
	summary, _ := fx.svc.Summary(project.ID)
	if summary.ApprovedAmount != 10000 || summary.PendingAmount != 0 || summary.RemainingAmount != 50000 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
}

func TestChangeOrderService_ApproveExceedingContractRejected(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 40000)
	node := fx.seedNode(project.ID)
	// 40000 + 60001 > 100000，超出合同额与已用预算的差额。
	order, err := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "大额增项", Amount: 60001})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := fx.svc.Review(order.ID, 9, &dto.ReviewChangeOrderRequest{Approved: true}); err == nil {
		t.Fatal("expected conflict when cumulative changes exceed contract headroom")
	}

	// 整单拒绝：仍为待审批，预算与汇总不变。
	got, _ := fx.svc.GetByID(order.ID)
	if got.Status != constants.ChangeOrderStatusPending {
		t.Fatalf("expected order to stay pending, got %q", got.Status)
	}
	used, _ := fx.budgets.SumActualByProjectID(project.ID)
	if used != 40000 {
		t.Fatalf("expected used budget unchanged 40000, got %v", used)
	}
	updated, _ := fx.projects.GetByID(project.ID)
	if updated.ApprovedChangeAmount != 0 {
		t.Fatalf("expected change summary 0, got %v", updated.ApprovedChangeAmount)
	}
}

func TestChangeOrderService_CumulativeChangesExceedingRejected(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 40000)
	nodeA := fx.seedNode(project.ID)
	nodeB := fx.seedNode(project.ID)
	first, _ := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: nodeA.ID, Title: "增项A", Amount: 50000})
	second, _ := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: nodeB.ID, Title: "增项B", Amount: 20000})

	if _, err := fx.svc.Review(first.ID, 9, &dto.ReviewChangeOrderRequest{Approved: true}); err != nil {
		t.Fatalf("approve first: %v", err)
	}
	// 已用预算变为 90000，累计变更 50000+20000 超出 100000-90000。
	if _, err := fx.svc.Review(second.ID, 9, &dto.ReviewChangeOrderRequest{Approved: true}); err == nil {
		t.Fatal("expected conflict for cumulative changes exceeding headroom")
	}
	got, _ := fx.svc.GetByID(second.ID)
	if got.Status != constants.ChangeOrderStatusPending {
		t.Fatalf("expected second order to stay pending, got %q", got.Status)
	}
}

func TestChangeOrderService_RejectKeepsBudget(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 40000)
	node := fx.seedNode(project.ID)
	order, _ := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "增项", Amount: 10000})

	got, err := fx.svc.Review(order.ID, 9, &dto.ReviewChangeOrderRequest{Approved: false, Note: "超出需求"})
	if err != nil {
		t.Fatalf("reject: %v", err)
	}
	if got.Status != constants.ChangeOrderStatusRejected {
		t.Fatalf("expected rejected, got %q", got.Status)
	}
	used, _ := fx.budgets.SumActualByProjectID(project.ID)
	if used != 40000 {
		t.Fatalf("expected used budget unchanged after reject, got %v", used)
	}
	updated, _ := fx.projects.GetByID(project.ID)
	if updated.ApprovedChangeAmount != 0 {
		t.Fatalf("expected change summary 0 after reject, got %v", updated.ApprovedChangeAmount)
	}

	// 驳回后该节点可再次提交。
	if _, err := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "增项v2", Amount: 8000}); err != nil {
		t.Fatalf("resubmit after reject: %v", err)
	}
}

func TestChangeOrderService_DuplicateReviewConflict(t *testing.T) {
	fx := newChangeOrderFixture()
	project := fx.seedProject(100000, 40000)
	node := fx.seedNode(project.ID)
	order, _ := fx.svc.Create(7, &dto.CreateChangeOrderRequest{ProjectID: project.ID, NodeID: node.ID, Title: "增项", Amount: 10000})

	if _, err := fx.svc.Review(order.ID, 9, &dto.ReviewChangeOrderRequest{Approved: true}); err != nil {
		t.Fatalf("first review: %v", err)
	}
	// 重复审批只允许首个成功。
	if _, err := fx.svc.Review(order.ID, 10, &dto.ReviewChangeOrderRequest{Approved: true}); err == nil {
		t.Fatal("expected conflict on duplicate approval")
	}
	if _, err := fx.svc.Review(order.ID, 10, &dto.ReviewChangeOrderRequest{Approved: false}); err == nil {
		t.Fatal("expected conflict on reject after approval")
	}

	// 预算只落账一次。
	used, _ := fx.budgets.SumActualByProjectID(project.ID)
	if used != 50000 {
		t.Fatalf("expected used budget 50000, got %v", used)
	}
	updated, _ := fx.projects.GetByID(project.ID)
	if updated.ApprovedChangeAmount != 10000 {
		t.Fatalf("expected change summary 10000, got %v", updated.ApprovedChangeAmount)
	}
}
