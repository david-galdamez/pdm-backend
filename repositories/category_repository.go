package repositories

import (
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

type CategoryRepository struct {
	DB *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{DB: db}
}

type FinanceCategory struct {
	CategoryID   uint   `json:"category_id"`
	CategoryName string `json:"category_name"`
}

func (r *CategoryRepository) GetCategories(financeId uint) ([]FinanceCategory, error) {

	var categories []FinanceCategory

	err := r.DB.Model(models.ExpenseCategory{}).Where("finance_id = ?", financeId).
		Select("expense_categories.id AS category_id, expense_categories.name AS category_name").
		Scan(&categories).Error

	if err != nil {
		return nil, err
	}

	return categories, err
}

type SubcategoryBreakdown struct {
	Name       string  `json:"subcategory_name"`
	Budget     float64 `json:"subcategory_budget"`
	Spent      float64 `json:"subcategory_spent"`
	Difference float64 `json:"subcategory_difference"`
}

type CategoryTotals struct {
	Budget float64
	Spent  float64
}

type CategoryBreakdown struct {
	Budget        float64                `json:"total_budget"`
	Spent         float64                `json:"total_spent"`
	Difference    float64                `json:"total_difference"`
	Subcategories []SubcategoryBreakdown `json:"subcategories"`
}

func (r *CategoryRepository) GetCategoriesData(financeId uint, categoryId *uint) (*CategoryBreakdown, error) {

	var totals CategoryTotals
	var subcategories []SubcategoryBreakdown
	errCh := make(chan error, 2)

	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	baseQuery := func(tx *gorm.DB) *gorm.DB {
		q := tx.Where("expense_subcategories.finance_id = ?", financeId)
		if categoryId != nil {
			q = q.Where("expense_subcategories.expense_category_id = ?", *categoryId)
		}
		return q
	}

	go func() {
		err := baseQuery(r.DB.Model(models.ExpenseSubcategory{})).
			Select(`
			CASE
				WHEN expense_subcategories.name = ? THEN MAX(monthly_goals.target_amount)
				ELSE COALESCE(SUM(expense_subcategories.monthly_budget),0)
			END AS budget,
			COALESCE(SUM(transactions.amount), 0) AS spent`, models.SavingsCategoryName).
			Joins("LEFT JOIN monthly_goals ON monthly_goals.finance_id = expense_subcategories.finance_id AND monthly_goals.month = ? AND monthly_goals.year = ?", month, year).
			Joins("LEFT JOIN transactions ON transactions.expense_subcategory_id = expense_subcategories.id").
			Group("expense_subcategories.id").Scan(&totals).Error
		errCh <- err
	}()

	go func() {
		err := baseQuery(r.DB.Model(models.ExpenseSubcategory{})).
			Select(`
			expense_subcategories.name AS name,
			CASE
				WHEN expense_subcategories.name = ? THEN MAX(monthly_goals.target_amount)
				ELSE COALESCE(SUM(expense_subcategories.monthly_budget),0)
			END AS budget,
			COALESCE(SUM(transactions.amount),0) AS spent`, models.SavingsCategoryName).
			Joins("LEFT JOIN transactions ON transactions.expense_subcategory_id = expense_subcategories.id").
			Joins("LEFT JOIN monthly_goals ON monthly_goals.finance_id = expense_subcategories.finance_id AND monthly_goals.month = ? AND monthly_goals.year = ?", month, year).
			Group("expense_subcategories.id").Scan(&subcategories).Error
		errCh <- err
	}()

	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			return nil, err
		}
	}

	for index := range subcategories {
		subcategories[index].Difference = subcategories[index].Budget - subcategories[index].Spent
	}

	breakdown := CategoryBreakdown{
		Budget:     totals.Budget,
		Spent:      totals.Spent,
		Difference: totals.Budget - totals.Spent,
	}

	if subcategories == nil {
		breakdown.Subcategories = []SubcategoryBreakdown{}
	} else {
		breakdown.Subcategories = subcategories
	}

	return &breakdown, nil
}

func (r *CategoryRepository) CreateCategory(category *models.ExpenseCategory) error {
	return r.DB.Create(&category).Error
}

func (r *CategoryRepository) GetCategoryById(id *uint) (*models.ExpenseCategory, error) {

	var category models.ExpenseCategory

	if err := r.DB.First(&category, id).Error; err != nil {
		return nil, err
	}

	return &category, nil
}

type CategoryListItem struct {
	CategoryID   uint   `json:"category_id"`
	CategoryName string `json:"category_name"`
	UserName     string `json:"user_name"`
}

func (r *CategoryRepository) GetCategoriesList(financeId uint) ([]CategoryListItem, error) {

	var categories []CategoryListItem

	err := r.DB.Model(models.ExpenseCategory{}).Where("finance_id = ?", financeId).
		Select("expense_categories.id AS category_id, expense_categories.name AS category_name, users.name AS user_name").
		Joins("LEFT JOIN users ON users.id = expense_categories.user_id").
		Scan(&categories).Error

	if err != nil {
		return nil, err
	}

	return categories, err
}

func (r *CategoryRepository) UpdateCategory(category *models.ExpenseCategory) error {
	return r.DB.Save(&category).Error
}
