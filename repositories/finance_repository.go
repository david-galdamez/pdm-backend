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

func SumAmount(db *gorm.DB, model interface{}, financeId uint, entryType int, from, to time.Time) (float64, error) {
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
		Select("SUM(CASE WHEN entry_type_id = 1 THEN amount ELSE 0 END) AS total_income, SUM(CASE WHEN entry_type_id = 2 THEN amount ELSE 0 END) AS total_expense").
		Where("finance_id = ? AND occurred_at >= ? AND occurred_at < ? AND deleted_at IS NULL", financeId, from, to).
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

	var totalExpense, monthlyBudget float64
	errCh := make(chan error, 2)

	go func() {
		err := r.DB.Model(models.ExpenseSubcategory{}).
			Where("finance_id = ?", financeId).
			Select("COALESCE(SUM(monthly_budget), 0)").
			Scan(&monthlyBudget).Error

		errCh <- err
	}()

	go func() {
		amount, err := SumAmount(r.DB, models.Transaction{}, financeId, int(models.EntryTypeExpense), from, to)

		if err == nil {
			totalExpense = amount
		}

		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	variance := monthlyBudget - totalExpense

	return gin.H{
		"monthly_budget":   monthlyBudget,
		"monthly_spent":    totalExpense,
		"monthly_variance": variance,
	}, nil
}

func (r *FinanceRepository) GetSavingsSummary(financeId uint, month, year int) (gin.H, error) {
	var targetAmount float64
	var savedAmount float64

	var goal models.MonthlyGoal
	errCh := make(chan error, 2)

	go func() {
		err := r.DB.Model(models.MonthlyGoal{}).Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
			First(&goal).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			errCh <- nil
			return
		}
		errCh <- err
	}()

	go func() {
		err := r.DB.Model(models.MonthlySaving{}).
			Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
			Select("amount").Scan(&savedAmount).Error
		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	targetAmount = goal.TargetAmount
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

	var results []DashboardData

	err := r.DB.Model(models.ExpenseCategory{}).Where("expense_categories.finance_id = ?", financeId).
		Select("expense_categories.id AS category_id, expense_categories.name AS category_name, COALESCE(SUM(expense_subcategories.monthly_budget), 0) AS total_budget").
		Joins("LEFT JOIN expense_subcategories ON expense_subcategories.expense_category_id = expense_categories.id").
		Group("expense_categories.id, expense_categories.name").
		Order("expense_categories.name").
		Scan(&results).Error
	if err != nil {
		return nil, err
	}

	month := int(monthStart.Month())
	year := monthStart.Year()

	for index := range results {

		results[index].FinanceID = financeId

		if results[index].CategoryName == models.SavingsCategoryName {
			var goal models.MonthlyGoal
			err := r.DB.Model(models.MonthlyGoal{}).
				Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
				First(&goal).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			var monthlySaving models.MonthlySaving
			err = r.DB.Model(models.MonthlySaving{}).
				Where("finance_id = ? AND month = ? AND year = ?", financeId, month, year).
				First(&monthlySaving).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}

			results[index].TotalBudget = goal.TargetAmount
			results[index].Spent = monthlySaving.Amount
			results[index].Difference = goal.TargetAmount - monthlySaving.Amount

			continue
		}

		var totalSpent float64

		err := r.DB.Model(models.Transaction{}).
			Where("finance_id = ? AND entry_type_id = ? AND occurred_at >= ? AND occurred_at < ? AND expense_category_id = ?",
				financeId, models.EntryTypeExpense, monthStart, monthEnd, results[index].CategoryID).
			Select("COALESCE(SUM(amount), 0)").Scan(&totalSpent).Error

		if err != nil {
			return nil, err
		}

		results[index].Spent = totalSpent
		results[index].Difference = results[index].TotalBudget - totalSpent
	}

	return results, nil
}
