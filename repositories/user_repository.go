package repositories

import (
	"pdm-backend/models"

	"gorm.io/gorm"
)

type UserRepository struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) Create(user *models.User) error {
	return r.DB.Create(&user).Error
}

func (r *UserRepository) CreateUserAndFinance(user *models.User) error {

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&user).Error; err != nil {
			return err
		}

		finance := models.Finance{
			UserID:        user.ID,
			FinanceTypeID: models.FinanceTypePersonal,
		}
		if err := tx.Create(&finance).Error; err != nil {
			return err
		}

		category := models.ExpenseCategory{
			FinanceID: finance.ID,
			Name:      models.SavingsCategoryName,
			UserID:    user.ID,
		}
		if err := tx.Create(&category).Error; err != nil {
			return err
		}

		subcategory := models.ExpenseSubcategory{
			FinanceID:         finance.ID,
			Name:              models.SavingsCategoryName,
			BudgetTypeID:      models.BudgetTypeProvisional,
			ExpenseCategoryID: category.ID,
			MonthlyBudget:     0.00,
			UserID:            user.ID,
		}
		if err := tx.Create(&subcategory).Error; err != nil {
			return err
		}

		return nil
	})

	return err
}

func (r *UserRepository) GetUserByEmail(email string) (*models.User, error) {
	var user models.User

	if err := r.DB.Where("email = ?", email).First(&user).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetUserById(id uint) (*models.User, error) {
	var user models.User

	if err := r.DB.First(&user, id).Error; err != nil {
		return nil, err
	}

	return &user, nil
}

type Identifiers struct {
	FinanceID uint
	SavingsID uint
}

func (r *UserRepository) GetFinanceAndSavingSubcategoryByUserId(userId uint) (*Identifiers, error) {
	var identifiers Identifiers

	err := r.DB.Model(&models.Finance{}).
		Select("finances.id AS finance_id, expense_subcategories.id AS savings_id").
		Joins("JOIN expense_subcategories ON finances.id = expense_subcategories.finance_id").
		Where("finances.user_id = ? AND finances.finance_type_id = ? AND expense_subcategories.name = ?",
			userId, models.FinanceTypePersonal, models.SavingsCategoryName).
		Scan(&identifiers).Error

	if err != nil {
		return nil, err
	}

	return &identifiers, nil
}

func (r *UserRepository) UpdateUser(user *models.User) error {
	return r.DB.Save(&user).Error
}
