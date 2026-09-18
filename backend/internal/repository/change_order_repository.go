package repository

import (
	"errors"
	"fmt"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/home-renovation/platform/internal/constants"
	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
)

// ErrDuplicatePending 哨兵错误：同一节点已存在待审批变更单。
var ErrDuplicatePending = errors.New("pending change order already exists for node")

// ChangeOrderRepository 变更签证仓储接口。
type ChangeOrderRepository interface {
	Create(order *model.ChangeOrder) error
	GetByID(id uint) (*model.ChangeOrder, error)
	List(filter ChangeOrderFilter, page, pageSize int) ([]model.ChangeOrder, int64, error)
	ListByProjectID(projectID uint, status string) ([]model.ChangeOrder, error)
	HasPendingByNodeID(nodeID uint) (bool, error)
	SumAmountByProjectAndStatus(projectID uint, status string) (float64, error)
	CountByProjectAndStatus(projectID uint, status string) (int64, error)
	// MarkReviewed 仅当变更单仍处于待审批时流转状态，返回是否流转成功。
	MarkReviewed(id uint, status string, reviewedBy uint, note string, reviewedAt time.Time) (bool, error)
	WithTx(tx *gorm.DB) ChangeOrderRepository
}

// ChangeOrderFilter 变更签证查询过滤条件。
type ChangeOrderFilter struct {
	ProjectID uint
	NodeID    uint
	Status    string
}

type changeOrderRepository struct {
	db *gorm.DB
}

// NewChangeOrderRepository 构造变更签证仓储。
func NewChangeOrderRepository(db *gorm.DB) ChangeOrderRepository {
	return &changeOrderRepository{db: db}
}

// WithTx 返回绑定到指定事务的仓储。
func (r *changeOrderRepository) WithTx(tx *gorm.DB) ChangeOrderRepository {
	return &changeOrderRepository{db: tx}
}

func (r *changeOrderRepository) Create(order *model.ChangeOrder) error {
	if err := r.db.Create(order).Error; err != nil {
		if isDuplicateKeyError(err) {
			return ErrDuplicatePending
		}
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

func (r *changeOrderRepository) ListByProjectID(projectID uint, status string) ([]model.ChangeOrder, error) {
	var orders []model.ChangeOrder
	query := r.db.Where("project_id = ?", projectID)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	if err := query.Order("id DESC").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("list change orders by project %d: %w", projectID, err)
	}
	return orders, nil
}

func (r *changeOrderRepository) HasPendingByNodeID(nodeID uint) (bool, error) {
	var count int64
	if err := r.db.Model(&model.ChangeOrder{}).
		Where("node_id = ? AND status = ?", nodeID, constants.ChangeOrderStatusPending).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("count pending change orders for node %d: %w", nodeID, err)
	}
	return count > 0, nil
}

func (r *changeOrderRepository) SumAmountByProjectAndStatus(projectID uint, status string) (float64, error) {
	var total float64
	if err := r.db.Model(&model.ChangeOrder{}).
		Where("project_id = ? AND status = ?", projectID, status).
		Select("COALESCE(SUM(amount), 0)").
		Scan(&total).Error; err != nil {
		return 0, fmt.Errorf("sum %s change orders for project %d: %w", status, projectID, err)
	}
	return total, nil
}

func (r *changeOrderRepository) CountByProjectAndStatus(projectID uint, status string) (int64, error) {
	var count int64
	if err := r.db.Model(&model.ChangeOrder{}).
		Where("project_id = ? AND status = ?", projectID, status).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count %s change orders for project %d: %w", status, projectID, err)
	}
	return count, nil
}

func (r *changeOrderRepository) MarkReviewed(id uint, status string, reviewedBy uint, note string, reviewedAt time.Time) (bool, error) {
	result := r.db.Model(&model.ChangeOrder{}).
		Where("id = ? AND status = ?", id, constants.ChangeOrderStatusPending).
		Updates(map[string]interface{}{
			"status":      status,
			"reviewed_by": reviewedBy,
			"review_note": note,
			"reviewed_at": reviewedAt,
		})
	if result.Error != nil {
		return false, fmt.Errorf("mark change order %d reviewed: %w", id, result.Error)
	}
	return result.RowsAffected > 0, nil
}

// isDuplicateKeyError 判断是否为 MySQL 唯一键冲突（待审批变更唯一索引）。
func isDuplicateKeyError(err error) bool {
	var mysqlErr *mysqldriver.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
