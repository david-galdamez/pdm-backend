package repositories

import (
	"pdm-backend/models"

	"gorm.io/gorm"
)

type IncomeSourceRepository struct {
	DB *gorm.DB
}

func NewIncomeSourceRepository(db *gorm.DB) *IncomeSourceRepository {
	return &IncomeSourceRepository{DB: db}
}

type IncomeSourceOption struct {
	IncomeSourceID uint    `json:"option_id"`
	Name           string  `json:"option_name"`
	Amount         float64 `json:"option_budget"`
}

func (r *IncomeSourceRepository) GetIncomeSources(financeId uint) ([]IncomeSourceOption, error) {

	options := []IncomeSourceOption{}

	err := r.DB.Model(models.IncomeSource{}).Where("finance_id = ?", financeId).
		Select("income_sources.id AS income_source_id, income_sources.name AS name, income_sources.amount AS amount").
		Scan(&options).Error

	if err != nil {
		return nil, err
	}

	return options, nil
}

type IncomeSourceListItem struct {
	FinanceID      uint    `json:"finance_id"`
	IncomeSourceID uint    `json:"income_source_id"`
	Name           string  `json:"name"`
	Amount         float64 `json:"amount"`
	UserName       string  `json:"user_name"`
}

func (r *IncomeSourceRepository) GetIncomeSourcesList(financeId uint) ([]IncomeSourceListItem, error) {

	list := []IncomeSourceListItem{}

	err := r.DB.Model(models.IncomeSource{}).Where("finance_id = ?", financeId).
		Select("income_sources.finance_id AS finance_id, income_sources.id AS income_source_id, income_sources.name AS name, income_sources.amount AS amount, users.name AS user_name").
		Joins("LEFT JOIN users ON users.id = income_sources.user_id").
		Scan(&list).Error

	if err != nil {
		return nil, err
	}

	return list, nil
}

func (r *IncomeSourceRepository) CreateIncomeSource(incomeSource *models.IncomeSource) error {
	return r.DB.Create(&incomeSource).Error
}

func (r *IncomeSourceRepository) GetIncomeSourceById(id *uint) (*models.IncomeSource, error) {
	var incomeSource models.IncomeSource

	if err := r.DB.First(&incomeSource, id).Error; err != nil {
		return nil, err
	}

	return &incomeSource, nil
}

type IncomeSourceResponse struct {
	IncomeSourceID uint    `json:"income_source_id"`
	Name           string  `json:"name"`
	Amount         float64 `json:"amount"`
	Description    string  `json:"description"`
}

func (r *IncomeSourceRepository) GetIncomeSource(id *uint) (*IncomeSourceResponse, error) {
	var response IncomeSourceResponse

	tx := r.DB.Model(models.IncomeSource{}).Where("income_sources.id = ?", id).
		Select("income_sources.id AS income_source_id, income_sources.name AS name, income_sources.amount AS amount, income_sources.description AS description").
		Scan(&response)

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	if err := tx.Error; err != nil {
		return nil, err
	}

	return &response, nil
}

func (r *IncomeSourceRepository) UpdateIncomeSource(incomeSource *models.IncomeSource) error {
	return r.DB.Save(&incomeSource).Error
}
