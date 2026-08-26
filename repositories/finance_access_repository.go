package repositories

import (
	"pdm-backend/models"

	"gorm.io/gorm"
)

type FinanceAccessRepository struct {
	DB *gorm.DB
}

func NewFinanceAccessRepository(db *gorm.DB) *FinanceAccessRepository {
	return &FinanceAccessRepository{DB: db}
}

// accessQuery matches the finance when the user may act on it: either it is the
// personal finance they own, or an active membership joins them to it. Split out
// from CanAccessFinance so the generated SQL can be asserted in a test.
func (r *FinanceAccessRepository) accessQuery(userId, financeId uint) *gorm.DB {

	membership := r.DB.Model(&models.SharedFinance{}).Select("1").
		Where("shared_finances.finance_id = finances.id AND shared_finances.user_id = ? AND shared_finances.active", userId)

	return r.DB.Model(&models.Finance{}).
		Where("finances.id = ?", financeId).
		Where(
			r.DB.Where("finances.user_id = ? AND finances.finance_type_id = ?", userId, models.FinanceTypePersonal).
				Or("EXISTS (?)", membership),
		)
}

// CanAccessFinance reports whether the user may act on the finance. This is the
// same rule the websocket handler enforces before accepting a connection.
func (r *FinanceAccessRepository) CanAccessFinance(userId, financeId uint) (bool, error) {

	var count int64

	if err := r.accessQuery(userId, financeId).Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}
