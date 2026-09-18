package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

func newChangeOrderTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.ChangeOrder{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestChangeOrderRepository_CreateAndGet(t *testing.T) {
	db := newChangeOrderTestDB(t)
	repo := NewChangeOrderRepository(db)
	order := &model.ChangeOrder{
		ProjectID: 1, NodeID: 2, Title: "增加插座", Amount: 5000, ScheduleImpactDays: 3,
		Status: constants.ChangeOrderStatusPending, SubmittedBy: 7,
	}
	if err := repo.Create(order); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := repo.GetByID(order.ID)
	if err != nil {
		t.Fatalf("get by id: %v", err)
	}
	if got.Title != order.Title || got.Amount != 5000 || got.Status != constants.ChangeOrderStatusPending {
		t.Fatalf("unexpected order: %+v", got)
	}
	if _, err := repo.GetByID(9999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestChangeOrderRepository_HasPendingByNodeID(t *testing.T) {
	db := newChangeOrderTestDB(t)
	repo := NewChangeOrderRepository(db)
	has, err := repo.HasPendingByNodeID(2)
	if err != nil || has {
		t.Fatalf("expected no pending, got %v %v", has, err)
	}
	order := &model.ChangeOrder{ProjectID: 1, NodeID: 2, Title: "增项", Amount: 100, Status: constants.ChangeOrderStatusPending}
	if err := repo.Create(order); err != nil {
		t.Fatalf("create: %v", err)
	}
	has, err = repo.HasPendingByNodeID(2)
	if err != nil || !has {
		t.Fatalf("expected pending, got %v %v", has, err)
	}
	// 审批后不再占用待审批名额。
	if _, err := repo.MarkReviewed(order.ID, constants.ChangeOrderStatusRejected, 9, "驳回", time.Now()); err != nil {
		t.Fatalf("mark reviewed: %v", err)
	}
	has, err = repo.HasPendingByNodeID(2)
	if err != nil || has {
		t.Fatalf("expected no pending after review, got %v %v", has, err)
	}
}

func TestChangeOrderRepository_MarkReviewedOnlyOnce(t *testing.T) {
	db := newChangeOrderTestDB(t)
	repo := NewChangeOrderRepository(db)
	order := &model.ChangeOrder{ProjectID: 1, NodeID: 2, Title: "增项", Amount: 100, Status: constants.ChangeOrderStatusPending}
	if err := repo.Create(order); err != nil {
		t.Fatalf("create: %v", err)
	}
	marked, err := repo.MarkReviewed(order.ID, constants.ChangeOrderStatusApproved, 9, "同意", time.Now())
	if err != nil || !marked {
		t.Fatalf("expected first review to succeed, got %v %v", marked, err)
	}
	// 重复或并发审批只允许首个成功。
	marked, err = repo.MarkReviewed(order.ID, constants.ChangeOrderStatusRejected, 10, "重复审批", time.Now())
	if err != nil {
		t.Fatalf("second review: %v", err)
	}
	if marked {
		t.Fatal("expected second review to be rejected")
	}
	got, _ := repo.GetByID(order.ID)
	if got.Status != constants.ChangeOrderStatusApproved || got.ReviewedBy != 9 {
		t.Fatalf("expected first review to win, got %+v", got)
	}
}

func TestChangeOrderRepository_SumAndCountByStatus(t *testing.T) {
	db := newChangeOrderTestDB(t)
	repo := NewChangeOrderRepository(db)
	orders := []model.ChangeOrder{
		{ProjectID: 1, NodeID: 1, Title: "A", Amount: 1000, Status: constants.ChangeOrderStatusPending},
		{ProjectID: 1, NodeID: 2, Title: "B", Amount: 2000, Status: constants.ChangeOrderStatusApproved},
		{ProjectID: 1, NodeID: 3, Title: "C", Amount: 3000, Status: constants.ChangeOrderStatusApproved},
		{ProjectID: 2, NodeID: 4, Title: "D", Amount: 9000, Status: constants.ChangeOrderStatusApproved},
	}
	for i := range orders {
		if err := repo.Create(&orders[i]); err != nil {
			t.Fatalf("create %d: %v", i, err)
		}
	}
	pending, err := repo.SumAmountByProjectAndStatus(1, constants.ChangeOrderStatusPending)
	if err != nil || pending != 1000 {
		t.Fatalf("expected pending 1000, got %v %v", pending, err)
	}
	approved, err := repo.SumAmountByProjectAndStatus(1, constants.ChangeOrderStatusApproved)
	if err != nil || approved != 5000 {
		t.Fatalf("expected approved 5000, got %v %v", approved, err)
	}
	count, err := repo.CountByProjectAndStatus(1, constants.ChangeOrderStatusApproved)
	if err != nil || count != 2 {
		t.Fatalf("expected approved count 2, got %d %v", count, err)
	}
}
