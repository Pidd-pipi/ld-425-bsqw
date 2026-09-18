package repository

import (
	"fmt"
	"math"

	"github.com/home-renovation/platform/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// changeAmountEpsilon 金额比较容差，消化 decimal 存储与浮点运算误差。
const changeAmountEpsilon = 0.005

// changeBudgetCategory 变更签证对应预算类别。
const changeBudgetCategory = "Other"

func clauseForUpdate() clause.Expression {
	return clause.Locking{Strength: "UPDATE"}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

// sumActualAmount 统计项目下全部预算项的实际花费累计。
// 使用锁定读，确保在并发审批场景读取到最新已提交的预算（项目行锁已串行化审批事务）。
func sumActualAmount(tx *gorm.DB, projectID uint) (float64, error) {
	var amounts []float64
	if err := forUpdate(tx).Model(&model.BudgetItem{}).
		Where("project_id = ?", projectID).
		Pluck("actual_amount", &amounts).Error; err != nil {
		return 0, fmt.Errorf("sum used budget for project %d: %w", projectID, err)
	}
	total := 0.0
	for _, amount := range amounts {
		total += amount
	}
	return round2(total), nil
}

// rejectCommentForOverBalance 超预算整单拒绝时的审批意见。
func rejectCommentForOverBalance(custom string) string {
	prefix := "累计变更额超过合同额与已用预算差额，整单拒绝"
	if custom == "" {
		return prefix
	}
	return fmt.Sprintf("%s：%s", prefix, custom)
}
