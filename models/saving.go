package models

import "gorm.io/gorm"

type MonthlyGoal struct {
	gorm.Model
	FinanceID    uint    `gorm:"index;not null" json:"finance_id"`
	Year         int     `gorm:"not null" json:"year"`
	Month        int     `gorm:"not null" json:"month"`
	TargetAmount float64 `gorm:"not null" json:"target_amount"`
	Finance      Finance `gorm:"foreignKey:FinanceID" json:"finance"`
}

type MonthlySaving struct {
	gorm.Model
	FinanceID uint    `gorm:"index;not null" json:"finance_id"`
	Year      int     `gorm:"not null" json:"year"`
	Month     int     `gorm:"not null" json:"month"`
	Amount    float64 `gorm:"not null" json:"amount"`
	Finance   Finance `gorm:"foreignKey:FinanceID" json:"finance"`
}
