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

// GetFinanceAndSavingSubcategoryByUserId resolves the ids that go into the
// user's token. Registration creates both, so a miss means the account is
// broken; reporting it beats minting a token carrying finance id 0.
func (r *UserRepository) GetFinanceAndSavingSubcategoryByUserId(userId uint) (*Identifiers, error) {
	var identifiers Identifiers

	tx := r.DB.Model(&models.Finance{}).
		Select("finances.id AS finance_id, expense_subcategories.id AS savings_id").
		Joins("JOIN expense_subcategories ON finances.id = expense_subcategories.finance_id AND expense_subcategories.deleted_at IS NULL").
		Where("finances.user_id = ? AND finances.finance_type_id = ? AND expense_subcategories.name = ?",
			userId, models.FinanceTypePersonal, models.SavingsCategoryName).
		Scan(&identifiers)

	if err := tx.Error; err != nil {
		return nil, err
	}

	if tx.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	return &identifiers, nil
}

func (r *UserRepository) UpdateUser(user *models.User) error {
	return r.DB.Save(&user).Error
}

// GetTokenVersion is what AuthMiddleware checks a token's embedded version
// against on every request, so a password change invalidates outstanding
// tokens without a revocation list.
func (r *UserRepository) GetTokenVersion(userId uint) (uint, error) {
	var user models.User

	if err := r.DB.Select("token_version").First(&user, userId).Error; err != nil {
		return 0, err
	}

	return user.TokenVersion, nil
}
