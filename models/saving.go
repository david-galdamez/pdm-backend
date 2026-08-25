package models

import "gorm.io/gorm"

// The unique indexes below are what make CreateOrUpdateSavingGoal and
// CreateOrUpdateSaving safe: without them two concurrent requests for the same
// period can both miss the lookup and insert competing rows. They are filtered
// on deleted_at so a soft-deleted period does not block a fresh one.

type MonthlyGoal struct {
	gorm.Model
	FinanceID    uint    `gorm:"not null;uniqueIndex:idx_monthly_goals_period,priority:1,where:deleted_at IS NULL" json:"finance_id"`
	Year         int     `gorm:"not null;uniqueIndex:idx_monthly_goals_period,priority:2" json:"year"`
	Month        int     `gorm:"not null;uniqueIndex:idx_monthly_goals_period,priority:3" json:"month"`
	TargetAmount float64 `gorm:"not null" json:"target_amount"`
	Finance      Finance `gorm:"foreignKey:FinanceID" json:"finance"`
}

type MonthlySaving struct {
	gorm.Model
	FinanceID uint    `gorm:"not null;uniqueIndex:idx_monthly_savings_period,priority:1,where:deleted_at IS NULL" json:"finance_id"`
	Year      int     `gorm:"not null;uniqueIndex:idx_monthly_savings_period,priority:2" json:"year"`
	Month     int     `gorm:"not null;uniqueIndex:idx_monthly_savings_period,priority:3" json:"month"`
	Amount    float64 `gorm:"not null" json:"amount"`
	Finance   Finance `gorm:"foreignKey:FinanceID" json:"finance"`
}
