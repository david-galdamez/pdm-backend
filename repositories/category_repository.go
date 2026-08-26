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

type CategoryBreakdown struct {
	Budget        float64                `json:"total_budget"`
	Spent         float64                `json:"total_spent"`
	Difference    float64                `json:"total_difference"`
	Subcategories []SubcategoryBreakdown `json:"subcategories"`
}

// GetCategoriesData breaks a category down into its subcategories with each
// one's budget and what has been spent against it this month.
//
// The spend is a correlated subquery rather than a join. Joining transactions
// before aggregating fans each subcategory out into one row per transaction,
// which multiplies SUM(monthly_budget) by the transaction count, and the
// category totals are then summed in Go instead of asking the database for a
// GROUP BY whose rows would have to be collapsed anyway.
func (r *CategoryRepository) GetCategoriesData(financeId uint, categoryId *uint) (*CategoryBreakdown, error) {

	subcategories := []SubcategoryBreakdown{}

	now := time.Now()
	month := int(now.Month())
	year := now.Year()
	monthStart := time.Date(year, now.Month(), 1, 0, 0, 0, 0, now.Location())
	monthEnd := monthStart.AddDate(0, 1, 0)

	query := r.DB.Model(models.ExpenseSubcategory{}).
		Where("expense_subcategories.finance_id = ?", financeId)

	if categoryId != nil {
		query = query.Where("expense_subcategories.expense_category_id = ?", *categoryId)
	}

	err := query.
		Select(`
		expense_subcategories.name AS name,
		CASE
			WHEN expense_subcategories.name = ? THEN COALESCE(monthly_goals.target_amount, 0)
			ELSE expense_subcategories.monthly_budget
		END AS budget,
		COALESCE((
			SELECT SUM(spend.amount)
			FROM transactions AS spend
			WHERE spend.expense_subcategory_id = expense_subcategories.id
			AND spend.entry_type_id = ?
			AND spend.occurred_at >= ?
			AND spend.occurred_at < ?
			AND spend.deleted_at IS NULL
		), 0) AS spent`, models.SavingsCategoryName, models.EntryTypeExpense, monthStart, monthEnd).
		Joins(`LEFT JOIN monthly_goals
			ON monthly_goals.finance_id = expense_subcategories.finance_id
			AND monthly_goals.month = ? AND monthly_goals.year = ?
			AND monthly_goals.deleted_at IS NULL`, month, year).
		Order("expense_subcategories.name").
		Scan(&subcategories).Error

	if err != nil {
		return nil, err
	}

	breakdown := CategoryBreakdown{Subcategories: subcategories}

	for index := range subcategories {
		subcategories[index].Difference = subcategories[index].Budget - subcategories[index].Spent

		breakdown.Budget += subcategories[index].Budget
		breakdown.Spent += subcategories[index].Spent
	}

	breakdown.Difference = breakdown.Budget - breakdown.Spent

	return &breakdown, nil
}

func (r *CategoryRepository) CreateCategory(category *models.ExpenseCategory) error {
	return r.DB.Create(&category).Error
}

func (r *CategoryRepository) GetCategoryById(id *uint, financeId uint) (*models.ExpenseCategory, error) {

	var category models.ExpenseCategory

	if err := r.DB.Where("id = ? AND finance_id = ?", *id, financeId).First(&category).Error; err != nil {
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
		Joins("LEFT JOIN users ON users.id = expense_categories.user_id AND users.deleted_at IS NULL").
		Scan(&categories).Error

	if err != nil {
		return nil, err
	}

	return categories, err
}

func (r *CategoryRepository) UpdateCategory(category *models.ExpenseCategory) error {
	return r.DB.Save(&category).Error
}
