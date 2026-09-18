package repository

import "gorm.io/gorm"

// TransactionManager 在单个数据库事务中执行一组操作。
type TransactionManager interface {
	InTransaction(fn func(tx *gorm.DB) error) error
}

type transactionManager struct {
	db *gorm.DB
}

// NewTransactionManager 构造事务管理器。
func NewTransactionManager(db *gorm.DB) TransactionManager {
	return &transactionManager{db: db}
}

func (m *transactionManager) InTransaction(fn func(tx *gorm.DB) error) error {
	return m.db.Transaction(fn)
}
