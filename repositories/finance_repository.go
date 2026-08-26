package repositories

import (
	"errors"
	"pdm-backend/models"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FinanceRepository struct {
	DB *gorm.DB
}

func NewFinanceRepository(db *gorm.DB) *FinanceRepository {
	return &FinanceRepository{DB: db}
}

// IsFinanceAdmin reports whether the user administers the finance: they
// created it, and it is a shared finance. There is no admin-transfer feature,
// so the creator is always the current admin.
func (r *FinanceRepository) IsFinanceAdmin(financeId, userId uint) (bool, error) {
	var count int64

	err := r.DB.Model(&models.Finance{}).
		Where("id = ? AND user_id = ? AND finance_type_id = ?", financeId, userId, models.FinanceTypeShared).
		Count(&count).Error
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

func SumAmount(db *gorm.DB, model any, financeId uint, entryType int, from, to time.Time) (float64, error) {
	var total float64

	err := db.Model(model).
		Where("finance_id = ? AND entry_type_id = ? AND occurred_at >= ? AND occurred_at < ?", financeId, entryType, from, to).
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error

	return total, err
}

type Summary struct {
	TotalIncome  float64 `json:"total_income" gorm:"column:total_income"`
	TotalExpense float64 `json:"total_expense" gorm:"column:total_expense"`
	Difference   float64 `json:"difference" gorm:"-"`
}

func (r *FinanceRepository) GetFinanceSummary(financeId uint, from, to time.Time) (gin.H, error) {

	var summary Summary
	err := r.DB.Model(&models.Transaction{}).
		Select("COALESCE(SUM(CASE WHEN entry_type_id = ? THEN amount ELSE 0 END), 0) AS total_income, COALESCE(SUM(CASE WHEN entry_type_id = ? THEN amount ELSE 0 END), 0) AS total_expense",
			models.EntryTypeIncome, models.EntryTypeExpense).
		Where("finance_id = ? AND occurred_at >= ? AND occurred_at < ?", financeId, from, to).
		Scan(&summary).Error
	if err != nil {
		return nil, err
	}

	summary.Difference = summary.TotalIncome - summary.TotalExpense

	return gin.H{
		"total_income":  summary.TotalIncome,
		"total_expense": summary.TotalExpense,
		"difference":    summary.Difference,
	}, nil
}

func (r *FinanceRepository) GetExpenseSummary(financeId uint, from, to time.Time) (gin.H, error) {

	var monthlyBudget float64

	err := r.DB.Model(models.ExpenseSubcategory{}).
		Where("finance_id = ?", financeId).
		Select("COALESCE(SUM(monthly_budget), 0)").
		Scan(&monthlyBudget).Error
	if err != nil {
		return nil, err
	}

	totalExpense, err := SumAmount(r.DB, models.Transaction{}, financeId, int(models.EntryTypeExpense), from, to)
	if err != nil {
		return nil, err
	}

	variance := monthlyBudget - totalExpense

	return gin.H{
		"monthly_budget":   monthlyBudget,
		"monthly_spent":    totalExpense,
		"monthly_variance": variance,
	}, nil
}

func (r *FinanceRepository) GetSavingsSummary(financeId uint, month, year int) (gin.H, error) {
	var savedAmount float64
	var goal models.MonthlyGoal

	err := r.DB.Model(models.MonthlyGoal{}).Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
		First(&goal).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	err = r.DB.Model(models.MonthlySaving{}).
		Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
		Select("COALESCE(amount, 0)").Scan(&savedAmount).Error
	if err != nil {
		return nil, err
	}

	targetAmount := goal.TargetAmount
	progress := 0.0

	if targetAmount != 0 {
		progress = (savedAmount * 100) / targetAmount
	}

	return gin.H{
		"goal":                targetAmount,
		"accumulated":         savedAmount,
		"progress_percentage": progress,
	}, nil
}

func (r FinanceRepository) GetDashboardSummary(financeId uint, monthStart, monthEnd time.Time) (gin.H, error) {

	var financeSummary gin.H
	var expenseSummary gin.H
	var savingsSummary gin.H

	errCh := make(chan error, 3)

	go func() {
		summary, err := r.GetFinanceSummary(financeId, monthStart, monthEnd)
		if err == nil {
			financeSummary = summary
		}
		errCh <- err

	}()

	go func() {
		summary, err := r.GetExpenseSummary(financeId, monthStart, monthEnd)
		if err == nil {
			expenseSummary = summary
		}
		errCh <- err
	}()

	go func() {
		summary, err := r.GetSavingsSummary(financeId, int(monthStart.Month()), monthStart.Year())
		if err == nil {
			savingsSummary = summary
		}
		errCh <- err
	}()

	for i := 0; i < 3; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	return gin.H{
		"finance_summary": financeSummary,
		"expense_summary": expenseSummary,
		"savings_summary": savingsSummary,
	}, nil

}

type DashboardData struct {
	FinanceID    uint    `json:"finance_id"`
	CategoryID   uint    `json:"-"`
	CategoryName string  `json:"category_name"`
	TotalBudget  float64 `json:"total_budget"`
	Spent        float64 `json:"spent"`
	Difference   float64 `json:"difference"`
}

func (r *FinanceRepository) GetDataSummary(monthStart, monthEnd time.Time, financeId uint) ([]DashboardData, error) {

	results := []DashboardData{}

	// Budget and spend are correlated subqueries rather than joins: joining
	// both would fan each category out into one row per subcategory per
	// transaction, and it keeps the whole breakdown to a single query instead
	// of one per category.
	err := r.DB.Model(models.ExpenseCategory{}).Where("expense_categories.finance_id = ?", financeId).
		Select(`
		expense_categories.id AS category_id,
		expense_categories.name AS category_name,
		COALESCE((
			SELECT SUM(sub.monthly_budget)
			FROM expense_subcategories AS sub
			WHERE sub.expense_category_id = expense_categories.id
			AND sub.deleted_at IS NULL
		), 0) AS total_budget,
		COALESCE((
			SELECT SUM(spend.amount)
			FROM transactions AS spend
			WHERE spend.expense_category_id = expense_categories.id
			AND spend.entry_type_id = ?
			AND spend.occurred_at >= ?
			AND spend.occurred_at < ?
			AND spend.deleted_at IS NULL
		), 0) AS spent`, models.EntryTypeExpense, monthStart, monthEnd).
		Order("expense_categories.name").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	month := int(monthStart.Month())
	year := monthStart.Year()

	// The savings category does not track a subcategory budget: its budget is
	// the month's goal and its spend is the month's accumulated saving.
	var savingsGoal, savingsAmount float64
	var needSavings bool

	for index := range results {
		if results[index].CategoryName == models.SavingsCategoryName {
			needSavings = true
			break
		}
	}

	if needSavings {
		err = r.DB.Model(models.MonthlyGoal{}).
			Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
			Select("COALESCE(target_amount, 0)").Scan(&savingsGoal).Error
		if err != nil {
			return nil, err
		}

		err = r.DB.Model(models.MonthlySaving{}).
			Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
			Select("COALESCE(amount, 0)").Scan(&savingsAmount).Error
		if err != nil {
			return nil, err
		}
	}

	for index := range results {
		results[index].FinanceID = financeId

		if results[index].CategoryName == models.SavingsCategoryName {
			results[index].TotalBudget = savingsGoal
			results[index].Spent = savingsAmount
		}

		results[index].Difference = results[index].TotalBudget - results[index].Spent
	}

	return results, nil
}
