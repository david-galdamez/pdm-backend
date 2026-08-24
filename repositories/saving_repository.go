package repositories

import (
	"errors"
	"pdm-backend/models"

	"gorm.io/gorm"
)

type SavingRepository struct {
	DB *gorm.DB
}

func NewSavingRepository(db *gorm.DB) *SavingRepository {
	return &SavingRepository{DB: db}
}

var monthNames = []string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

type SavingResponse struct {
	Month           int     `json:"month"`
	MonthName       string  `json:"month_name"`
	Year            int     `json:"year"`
	Goal            float64 `json:"goal"`
	SavedAmount     float64 `json:"saved_amount"`
	CompletionRatio float64 `json:"completion_percentage"`
}

func (r *SavingRepository) GetSavingsData(financeId uint, year int) ([]SavingResponse, error) {

	var rows []struct {
		Month        int
		TargetAmount float64
		SavedAmount  float64
	}

	err := r.DB.Model(models.MonthlyGoal{}).
		Select(`
			monthly_goals.month,
			monthly_goals.year,
			monthly_goals.target_amount,
			COALESCE(monthly_savings.amount, 0) AS saved_amount
		`).
		Joins(`
			LEFT JOIN monthly_savings
			ON monthly_goals.finance_id = monthly_savings.finance_id
			AND monthly_goals.month = monthly_savings.month
			AND monthly_goals.year = monthly_savings.year
		`).
		Where("monthly_goals.finance_id = ? AND monthly_goals.year = ?", financeId, year).
		Order("monthly_goals.month ASC").
		Scan(&rows).Error

	if err != nil {
		return nil, err
	}

	savings := []SavingResponse{}
	for _, row := range rows {
		percentage := 0.0
		if row.TargetAmount != 0 {
			percentage = (row.SavedAmount / row.TargetAmount) * 100
		}

		savings = append(savings, SavingResponse{
			Month:           row.Month,
			MonthName:       monthNames[row.Month-1],
			Year:            year,
			Goal:            row.TargetAmount,
			SavedAmount:     row.SavedAmount,
			CompletionRatio: percentage,
		})
	}

	return savings, nil
}

func (r *SavingRepository) CreateOrUpdateSavingGoal(financeId uint, amount float64, month, year int) error {

	var goal models.MonthlyGoal
	err := r.DB.Model(models.MonthlyGoal{}).
		Where("finance_id = ? AND year = ? AND month = ?", financeId, year, month).
		First(&goal).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newGoal := models.MonthlyGoal{
				FinanceID:    financeId,
				TargetAmount: amount,
				Month:        month,
				Year:         year,
			}

			return r.DB.Create(&newGoal).Error
		}
		return err
	}

	goal.TargetAmount = amount

	return r.DB.Save(&goal).Error
}
