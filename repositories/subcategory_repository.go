package repositories

import (
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

type SubcategoryRepository struct {
	DB *gorm.DB
}

func NewSubcategoryRepository(db *gorm.DB) *SubcategoryRepository {
	return &SubcategoryRepository{DB: db}
}

type FinanceSubcategory struct {
	SubcategoryID   uint    `json:"option_id"`
	SubcategoryName string  `json:"option_name"`
	Budget          float64 `json:"option_budget"`
}

func (r *SubcategoryRepository) GetSubcategories(financeId uint) ([]FinanceSubcategory, error) {

	var subcategories []FinanceSubcategory
	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	err := r.DB.Model(models.ExpenseSubcategory{}).Where("expense_subcategories.finance_id = ?", financeId).
		Select(`
		expense_subcategories.id AS subcategory_id,
		expense_subcategories.name AS subcategory_name,
		CASE
			WHEN expense_subcategories.name = ? THEN monthly_goals.target_amount
			ELSE expense_subcategories.monthly_budget
		END AS budget
		`, models.SavingsCategoryName).
		Joins(`LEFT JOIN monthly_goals ON monthly_goals.finance_id = expense_subcategories.finance_id
		AND monthly_goals.month = ? AND monthly_goals.year = ?
		`, month, year).
		Scan(&subcategories).Error

	if err != nil {
		return nil, err
	}

	return subcategories, err
}

type BudgetTypeOption struct {
	TypeID   uint   `json:"type_id"`
	TypeName string `json:"type_name"`
}

func (r *SubcategoryRepository) GetBudgetTypes() ([]BudgetTypeOption, error) {

	var options []BudgetTypeOption

	err := r.DB.Model(models.BudgetType{}).
		Select("budget_types.id AS type_id, budget_types.name AS type_name").
		Scan(&options).Error
	if err != nil {
		return nil, err
	}

	return options, nil
}

type SubcategoryListItem struct {
	FinanceID       uint    `json:"finance_id"`
	SubcategoryID   uint    `json:"subcategory_id"`
	CategoryName    string  `json:"category_name"`
	SubcategoryName string  `json:"subcategory_name"`
	BudgetTypeName  string  `json:"budget_type"`
	Budget          float64 `json:"budget"`
	UserName        string  `json:"user_name"`
}

func (r *SubcategoryRepository) GetSubcategoriesList(financeId uint) ([]SubcategoryListItem, error) {
	var subcategories []SubcategoryListItem

	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	err := r.DB.Model(models.ExpenseSubcategory{}).Where("expense_subcategories.finance_id = ?", financeId).
		Select(`expense_subcategories.finance_id AS finance_id,
		expense_subcategories.id AS subcategory_id,
		expense_categories.name AS category_name,
		expense_subcategories.name AS subcategory_name,
		budget_types.name AS budget_type_name,
		CASE
			WHEN expense_subcategories.name = ? THEN monthly_goals.target_amount
			ELSE expense_subcategories.monthly_budget
		END AS budget,
		users.name AS user_name`, models.SavingsCategoryName).
		Joins("LEFT JOIN expense_categories ON expense_subcategories.expense_category_id = expense_categories.id").
		Joins("LEFT JOIN monthly_goals ON monthly_goals.finance_id = expense_subcategories.finance_id AND monthly_goals.month = ? AND monthly_goals.year = ?", month, year).
		Joins("LEFT JOIN budget_types ON expense_subcategories.budget_type_id = budget_types.id").
		Joins("LEFT JOIN users ON expense_subcategories.user_id = users.id").
		Scan(&subcategories).Error

	if err != nil {
		return nil, err
	}

	return subcategories, nil
}

func (r *SubcategoryRepository) CreateSubcategory(subcategory *models.ExpenseSubcategory) error {
	return r.DB.Create(&subcategory).Error
}

func (r *SubcategoryRepository) GetSubcategoryById(id *uint) (*models.ExpenseSubcategory, error) {

	var subcategory models.ExpenseSubcategory

	if err := r.DB.First(&subcategory, id).Error; err != nil {
		return nil, err
	}

	return &subcategory, nil
}

type SubcategoryResponse struct {
	CategoryID   uint    `json:"category_id"`
	Name         string  `json:"name"`
	BudgetTypeID uint    `json:"budget_type_id"`
	Budget       float64 `json:"budget"`
}

func (r *SubcategoryRepository) GetSubcategory(id *uint) (*SubcategoryResponse, error) {

	var subcategory SubcategoryResponse

	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	tx := r.DB.Model(models.ExpenseSubcategory{}).Where("expense_subcategories.id = ?", id).
		Select(`
		expense_subcategories.expense_category_id AS category_id,
		expense_subcategories.name AS name,
		expense_subcategories.budget_type_id AS budget_type_id,
		CASE
			WHEN expense_subcategories.name = ? THEN monthly_goals.target_amount
			ELSE expense_subcategories.monthly_budget
		END AS budget
		`, models.SavingsCategoryName).
		Joins("LEFT JOIN monthly_goals ON monthly_goals.finance_id = expense_subcategories.finance_id AND monthly_goals.month = ? AND monthly_goals.year = ?", month, year).
		Scan(&subcategory)

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	if err := tx.Error; err != nil {
		return nil, err
	}

	return &subcategory, nil
}

func (r *SubcategoryRepository) UpdateSubcategory(subcategory *models.ExpenseSubcategory) error {
	return r.DB.Save(&subcategory).Error
}
