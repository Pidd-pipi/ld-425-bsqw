package repository

import (
	"errors"
	"fmt"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// BudgetRepository 预算项仓储接口。
type BudgetRepository interface {
	Create(item *model.BudgetItem) error
	GetByID(id uint) (*model.BudgetItem, error)
	List(filter BudgetFilter, page, pageSize int) ([]model.BudgetItem, int64, error)
	ListByProjectID(projectID uint) ([]model.BudgetItem, error)
	SumActualByProjectID(projectID uint) (float64, error)
	SumActualByProjectIDForUpdate(projectID uint) (float64, error)
	Update(item *model.BudgetItem) error
	Delete(id uint) error
	WithTx(tx *gorm.DB) BudgetRepository
}

// BudgetFilter 预算查询过滤条件。
type BudgetFilter struct {
	ProjectID uint
	Category  string
}

type budgetRepository struct {
	db *gorm.DB
}

// NewBudgetRepository 构造预算仓储。
func NewBudgetRepository(db *gorm.DB) BudgetRepository {
	return &budgetRepository{db: db}
}

// WithTx 返回绑定到指定事务的仓储。
func (r *budgetRepository) WithTx(tx *gorm.DB) BudgetRepository {
	return &budgetRepository{db: tx}
}

func (r *budgetRepository) Create(item *model.BudgetItem) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("create budget item: %w", err)
	}
	return nil
}

func (r *budgetRepository) GetByID(id uint) (*model.BudgetItem, error) {
	var item model.BudgetItem
	if err := r.db.First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get budget item %d: %w", id, err)
	}
	return &item, nil
}

func (r *budgetRepository) List(filter BudgetFilter, page, pageSize int) ([]model.BudgetItem, int64, error) {
	var items []model.BudgetItem
	var total int64
	query := r.db.Model(&model.BudgetItem{})
	if filter.ProjectID != 0 {
		query = query.Where("project_id = ?", filter.ProjectID)
	}
	if filter.Category != "" {
		query = query.Where("category = ?", filter.Category)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count budget items: %w", err)
	}
	offset := (page - 1) * pageSize
	if err := query.Order("id DESC").Offset(offset).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, fmt.Errorf("list budget items: %w", err)
	}
	return items, total, nil
}

func (r *budgetRepository) ListByProjectID(projectID uint) ([]model.BudgetItem, error) {
	var items []model.BudgetItem
	if err := r.db.Where("project_id = ?", projectID).Order("id ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list budget items by project %d: %w", projectID, err)
	}
	return items, nil
}

// SumActualByProjectID 汇总项目已用预算（实际花费合计）。
func (r *budgetRepository) SumActualByProjectID(projectID uint) (float64, error) {
	var total float64
	if err := r.db.Model(&model.BudgetItem{}).
		Where("project_id = ?", projectID).
		Select("COALESCE(SUM(actual_amount), 0)").
		Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("sum actual budget for project %d: %w", projectID, err)
	}
	return total, nil
}

// SumActualByProjectIDForUpdate 加行锁汇总已用预算。
// 锁定读取始终返回最新已提交数据，保证审批事务在持有项目锁后读到准确余额。
func (r *budgetRepository) SumActualByProjectIDForUpdate(projectID uint) (float64, error) {
	var items []model.BudgetItem
	if err := r.db.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("project_id = ?", projectID).
		Find(&items).Error; err != nil {
		return 0, fmt.Errorf("lock budget items for project %d: %w", projectID, err)
	}
	var total float64
	for _, item := range items {
		total += item.ActualAmount
	}
	return total, nil
}

func (r *budgetRepository) Update(item *model.BudgetItem) error {
	if err := r.db.Save(item).Error; err != nil {
		return fmt.Errorf("update budget item %d: %w", item.ID, err)
	}
	return nil
}

func (r *budgetRepository) Delete(id uint) error {
	if err := r.db.Delete(&model.BudgetItem{}, id).Error; err != nil {
		return fmt.Errorf("delete budget item %d: %w", id, err)
	}
	return nil
}
