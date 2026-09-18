package repository

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

func newChangeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&model.RenovationProject{},
		&model.ConstructionNode{},
		&model.BudgetItem{},
		&model.ChangeOrder{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// SQLite 内存库每个连接持有独立数据，测试中固定单连接串行化事务；
	// MySQL 生产环境通过 FOR UPDATE 行锁保证并发语义。
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	return db
}

func seedChangeFixture(t *testing.T, db *gorm.DB, contract, used float64) (*model.RenovationProject, *model.ConstructionNode) {
	t.Helper()
	project := &model.RenovationProject{
		Name: "测试项目", HouseType: "三室", Area: 100, DecorStyle: "Modern",
		Status: "InProgress", ContractAmount: contract,
		ExpectedEndDate: ptrTime(time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)),
	}
	if err := db.Create(project).Error; err != nil {
		t.Fatalf("create project: %v", err)
	}
	if used > 0 {
		budget := &model.BudgetItem{ProjectID: project.ID, Category: "Labor", BudgetAmount: used, ActualAmount: used}
		if err := db.Create(budget).Error; err != nil {
			t.Fatalf("create budget: %v", err)
		}
	}
	node := &model.ConstructionNode{ProjectID: project.ID, Name: "水电", Status: "InProgress", AcceptanceStatus: "Pending"}
	if err := db.Create(node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	return project, node
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestChangeOrderRepository_SubmitCreatesPending(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	_, node := seedChangeFixture(t, db, 100000, 10000)

	order := &model.ChangeOrder{
		ProjectID: node.ProjectID, NodeID: node.ID, Amount: 5000, ScheduleImpact: 2,
		Reason: "增项", Status: string(model.StatusPendingChange), ApplicantID: 3,
	}
	if err := repo.SubmitInTx(order); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if order.ID == 0 {
		t.Fatal("expected order id assigned")
	}
	got, err := repo.GetByID(order.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "Pending" {
		t.Fatalf("expected Pending, got %q", got.Status)
	}
}

func TestChangeOrderRepository_SubmitDuplicatePendingRejected(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	_, node := seedChangeFixture(t, db, 100000, 10000)

	first := &model.ChangeOrder{ProjectID: node.ProjectID, NodeID: node.ID, Amount: 1000, Reason: "a", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(first); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	second := &model.ChangeOrder{ProjectID: node.ProjectID, NodeID: node.ID, Amount: 2000, Reason: "b", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(second); !errors.Is(err, ErrPendingChangeExists) {
		t.Fatalf("expected ErrPendingChangeExists, got %v", err)
	}

	// 前序变更被批准后，节点可再次提交新变更。
	approved, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: first.ID, ReviewerID: 5, Approved: true, Now: time.Now()})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if approved.Status != "Approved" {
		t.Fatalf("expected Approved, got %q", approved.Status)
	}
	third := &model.ChangeOrder{ProjectID: node.ProjectID, NodeID: node.ID, Amount: 3000, Reason: "c", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(third); err != nil {
		t.Fatalf("submit after approved: %v", err)
	}
}

func TestChangeOrderRepository_ApproveAppliesBudgetAndSummary(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	project, node := seedChangeFixture(t, db, 100000, 10000)

	order := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 8000, ScheduleImpact: 5, Reason: "增项", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(order); err != nil {
		t.Fatalf("submit: %v", err)
	}

	got, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: order.ID, ReviewerID: 5, Approved: true, Comment: "同意", Now: time.Now()})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.Status != "Approved" || got.ReviewerID != 5 {
		t.Fatalf("unexpected order: %+v", got)
	}

	var updated model.RenovationProject
	if err := db.First(&updated, project.ID).Error; err != nil {
		t.Fatalf("get project: %v", err)
	}
	if updated.ApprovedChanges != 8000 {
		t.Fatalf("expected approved changes 8000, got %v", updated.ApprovedChanges)
	}
	if updated.ScheduleDelta != 5 {
		t.Fatalf("expected schedule delta 5, got %d", updated.ScheduleDelta)
	}
	wantEnd := time.Date(2027, 1, 5, 0, 0, 0, 0, time.UTC)
	if updated.ExpectedEndDate == nil || !updated.ExpectedEndDate.Equal(wantEnd) {
		t.Fatalf("expected end date %v, got %v", wantEnd, updated.ExpectedEndDate)
	}

	var budgets []model.BudgetItem
	if err := db.Where("project_id = ?", project.ID).Find(&budgets).Error; err != nil {
		t.Fatalf("list budgets: %v", err)
	}
	// 种子已用预算 1 条 + 变更生成 1 条。
	if len(budgets) != 2 {
		t.Fatalf("expected 2 budget items, got %d", len(budgets))
	}
	var changeBudget *model.BudgetItem
	for i := range budgets {
		if budgets[i].Category == "Other" {
			changeBudget = &budgets[i]
		}
	}
	if changeBudget == nil || changeBudget.ActualAmount != 8000 || changeBudget.BudgetAmount != 8000 {
		t.Fatalf("expected change budget item, got %+v", changeBudget)
	}
}

func TestChangeOrderRepository_RejectKeepsBudgetUnchanged(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	project, node := seedChangeFixture(t, db, 100000, 10000)

	order := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 8000, ScheduleImpact: 5, Reason: "增项", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(order); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: order.ID, ReviewerID: 5, Approved: false, Comment: "不同意", Now: time.Now()})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.Status != "Rejected" || got.ReviewComment != "不同意" {
		t.Fatalf("unexpected rejected order: %+v", got)
	}

	var updated model.RenovationProject
	if err := db.First(&updated, project.ID).Error; err != nil {
		t.Fatalf("get project: %v", err)
	}
	if updated.ApprovedChanges != 0 || updated.ScheduleDelta != 0 {
		t.Fatalf("expected unchanged summary, got %+v", updated)
	}
	var budgetCount int64
	if err := db.Model(&model.BudgetItem{}).Where("project_id = ?", project.ID).Count(&budgetCount).Error; err != nil {
		t.Fatalf("count budgets: %v", err)
	}
	if budgetCount != 1 {
		t.Fatalf("expected budget count 1 (unchanged), got %d", budgetCount)
	}
}

func TestChangeOrderRepository_OverBalanceWholeOrderRejected(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	// 合同 10 万，已用 9.5 万，剩余 5000；申请 8000 必须整单拒绝。
	project, node := seedChangeFixture(t, db, 100000, 95000)

	order := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 8000, ScheduleImpact: 3, Reason: "超支增项", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(order); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: order.ID, ReviewerID: 5, Approved: true, Now: time.Now()})
	if err != nil {
		t.Fatalf("review: %v", err)
	}
	if got.Status != "Rejected" {
		t.Fatalf("expected Rejected for over-balance change, got %q", got.Status)
	}
	if got.ReviewComment == "" {
		t.Fatal("expected rejection comment explaining balance overflow")
	}

	var updated model.RenovationProject
	if err := db.First(&updated, project.ID).Error; err != nil {
		t.Fatalf("get project: %v", err)
	}
	if updated.ApprovedChanges != 0 {
		t.Fatalf("expected no approved changes, got %v", updated.ApprovedChanges)
	}
	var budgetCount int64
	if err := db.Model(&model.BudgetItem{}).Where("project_id = ?", project.ID).Count(&budgetCount).Error; err != nil {
		t.Fatalf("count budgets: %v", err)
	}
	if budgetCount != 1 {
		t.Fatalf("expected budget count unchanged, got %d", budgetCount)
	}
}

func TestChangeOrderRepository_ConsecutiveApprovalsRespectCumulativeBalance(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	project, node := seedChangeFixture(t, db, 100000, 95000)

	// 第一次批准 3000，剩余空间 2000。
	first := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 3000, Reason: "a", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(first); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if _, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: first.ID, ReviewerID: 5, Approved: true, Now: time.Now()}); err != nil {
		t.Fatalf("first review: %v", err)
	}
	// 第二次 3000 超过累计空间（2000），应整单拒绝。
	second := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 3000, Reason: "b", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(second); err != nil {
		t.Fatalf("second submit: %v", err)
	}
	got, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: second.ID, ReviewerID: 5, Approved: true, Now: time.Now()})
	if err != nil {
		t.Fatalf("second review: %v", err)
	}
	if got.Status != "Rejected" {
		t.Fatalf("expected second change rejected, got %q", got.Status)
	}
}

func TestChangeOrderRepository_ConcurrentReviewOnlyFirstSucceeds(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	project, node := seedChangeFixture(t, db, 100000, 10000)
	order := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 1000, Reason: "并发审批", Status: "Pending", ApplicantID: 3}
	if err := repo.SubmitInTx(order); err != nil {
		t.Fatalf("submit: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	results := make(chan error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := repo.ReviewInTx(ChangeReviewInput{ChangeID: order.ID, ReviewerID: 5, Approved: true, Now: time.Now()})
			results <- err
		}()
	}
	wg.Wait()
	close(results)

	var success, alreadyReviewed int
	for err := range results {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrAlreadyReviewed):
			alreadyReviewed++
		default:
			t.Fatalf("unexpected review error: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("expected exactly 1 success, got %d", success)
	}
	if alreadyReviewed != workers-1 {
		t.Fatalf("expected %d conflicts, got %d", workers-1, alreadyReviewed)
	}
}

func TestChangeOrderRepository_ListFilters(t *testing.T) {
	db := newChangeTestDB(t)
	repo := NewChangeOrderRepository(db)
	project, node := seedChangeFixture(t, db, 100000, 10000)
	order := &model.ChangeOrder{ProjectID: project.ID, NodeID: node.ID, Amount: 1000, Reason: "a", Status: "Pending", ApplicantID: 3}
	if err := repo.Create(order); err != nil {
		t.Fatalf("create: %v", err)
	}
	orders, total, err := repo.List(ChangeOrderFilter{ProjectID: project.ID, Status: "Pending"}, 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(orders) != 1 {
		t.Fatalf("expected 1 pending order, got total=%d len=%d", total, len(orders))
	}
	byProject, err := repo.ListByProjectID(project.ID)
	if err != nil || len(byProject) != 1 {
		t.Fatalf("list by project: orders=%v err=%v", byProject, err)
	}
}
