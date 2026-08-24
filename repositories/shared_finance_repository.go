package repositories

import (
	"errors"
	"pdm-backend/models"
	"time"

	"gorm.io/gorm"
)

var (
	ErrAlreadyMember = errors.New("you already belong to this shared finance")
	ErrInviteExpired = errors.New("the invitation code has expired")
)

type SharedFinanceRepository struct {
	DB *gorm.DB
}

func NewSharedFinanceRepository(db *gorm.DB) *SharedFinanceRepository {
	return &SharedFinanceRepository{DB: db}
}

func (r *SharedFinanceRepository) DoesSharedFinanceExists(financeId uint) bool {
	var sharedFinance models.Finance
	err := r.DB.Where("id = ? AND finance_type_id = ?", financeId, models.FinanceTypeShared).First(&sharedFinance).Error
	return err == nil
}

func (r *SharedFinanceRepository) UserBelongsToSharedFinance(userId, financeId uint) bool {
	var sharedFinance models.SharedFinance
	err := r.DB.Where("finance_id = ? AND user_id = ? AND active = ?", financeId, userId, true).First(&sharedFinance).Error
	return err == nil
}

func (r *SharedFinanceRepository) CreateSharedFinance(userId uint, title, description string) error {

	err := r.DB.Transaction(func(tx *gorm.DB) error {
		finance := models.Finance{
			UserID:        userId,
			FinanceTypeID: models.FinanceTypeShared,
			Title:         &title,
			Description:   &description,
		}
		if err := tx.Create(&finance).Error; err != nil {
			return err
		}

		category := models.ExpenseCategory{
			FinanceID: finance.ID,
			Name:      models.SavingsCategoryName,
			UserID:    userId,
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
			UserID:            userId,
		}
		if err := tx.Create(&subcategory).Error; err != nil {
			return err
		}

		sharedFinance := models.SharedFinance{
			FinanceID: finance.ID,
			UserID:    userId,
			RoleID:    models.RoleAdmin,
			Active:    true,
			JoinedAt:  time.Now(),
		}
		if err := tx.Create(&sharedFinance).Error; err != nil {
			return err
		}

		return nil
	})

	return err
}

func (r *SharedFinanceRepository) JoinUser(userId uint, code string) error {

	var invitation models.Invitation

	// Joined so a stale or maliciously issued invitation cannot be redeemed
	// against a personal finance, even if one was somehow created for one.
	// invitations.* keeps the select unambiguous: both tables have id,
	// created_at, updated_at, and deleted_at columns.
	err := r.DB.Select("invitations.*").
		Joins("JOIN finances ON finances.id = invitations.finance_id AND finances.finance_type_id = ?", models.FinanceTypeShared).
		Where("code = ?", code).First(&invitation).Error
	if err != nil {
		return err
	}

	var existing models.SharedFinance
	err = r.DB.Where("finance_id = ? AND user_id = ? AND active = ?", invitation.FinanceID, userId, true).First(&existing).Error
	if err == nil {
		return ErrAlreadyMember
	}

	if time.Now().Before(invitation.ExpiresAt) {
		var inactive models.SharedFinance
		err = r.DB.Where("finance_id = ? AND user_id = ? AND active = ?", invitation.FinanceID, userId, false).First(&inactive).Error
		if err == nil {
			inactive.Active = true
			inactive.JoinedAt = time.Now()

			err := r.DB.Save(&inactive).Error
			if err != nil {
				return err
			}
		} else {
			sharedFinance := models.SharedFinance{
				FinanceID: invitation.FinanceID,
				UserID:    userId,
				RoleID:    models.RoleCollaborator,
				Active:    true,
				JoinedAt:  time.Now(),
			}

			err := r.DB.Create(&sharedFinance).Error
			if err != nil {
				return err
			}
		}
	} else {
		return ErrInviteExpired
	}

	return nil
}

type SharedFinanceListItem struct {
	FinanceID   uint   `json:"finance_id"`
	FinanceName string `json:"finance_name"`
	AdminName   string `json:"admin_name"`
}

func (r *SharedFinanceRepository) GetSharedFinances(userId uint) ([]SharedFinanceListItem, error) {

	finances := []SharedFinanceListItem{}

	err := r.DB.Model(models.SharedFinance{}).Where("shared_finances.user_id = ? AND shared_finances.active = ?", userId, true).
		Select("finances.id AS finance_id, finances.title AS finance_name, admin_users.name AS admin_name").
		Joins("INNER JOIN finances ON finances.id = shared_finances.finance_id").
		Joins("LEFT JOIN shared_finances AS admin_shared ON admin_shared.finance_id = finances.id AND admin_shared.role_id = ?", models.RoleAdmin).
		Joins("LEFT JOIN users AS admin_users ON admin_users.id = admin_shared.user_id").
		Scan(&finances).Error
	if err != nil {
		return nil, err
	}

	return finances, nil
}

type FinanceMember struct {
	UserID   uint   `json:"user_id"`
	UserName string `json:"user_name"`
	RoleID   uint   `json:"role_id"`
}

type SharedFinanceDetails struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Members     []FinanceMember `json:"members" gorm:"-"`
}

func (r *SharedFinanceRepository) GetSharedFinanceDetails(financeId uint) (*SharedFinanceDetails, error) {
	var details SharedFinanceDetails
	var members []FinanceMember

	err := r.DB.Model(models.Finance{}).
		Select("finances.title AS title, finances.description AS description").
		Where("finances.id = ?", financeId).
		Scan(&details).Error
	if err != nil {
		return nil, err
	}

	err = r.DB.Model(models.SharedFinance{}).
		Select("shared_finances.user_id AS user_id, users.name AS user_name, shared_finances.role_id AS role_id").
		Joins("JOIN users ON users.id = shared_finances.user_id").
		Where("shared_finances.finance_id = ? AND shared_finances.active = ?", financeId, true).
		Scan(&members).Error
	if err != nil {
		return nil, err
	}

	details.Members = members
	return &details, nil
}

func (r *SharedFinanceRepository) LeaveSharedFinance(userId, financeId uint) error {

	var sharedFinance models.SharedFinance

	err := r.DB.Model(models.SharedFinance{}).Where("finance_id = ? AND user_id = ?", financeId, userId).
		First(&sharedFinance).Error
	if err != nil {
		return err
	}

	sharedFinance.Active = false

	return r.DB.Save(&sharedFinance).Error
}
