package models

import "gorm.io/gorm"

type IncomeSource struct {
	gorm.Model
	FinanceID    uint    `json:"finance_id" gorm:"index;not null"`
	Finance      Finance `gorm:"foreignKey:FinanceID"`
	Name         string  `json:"name" gorm:"not null"`
	Amount       float64 `json:"amount" gorm:"not null"`
	Description  string  `json:"description" gorm:"size:500;not null"`
	UserID       uint    `json:"created_by_user_id" gorm:"index"`
	User         User    `gorm:"foreignKey:UserID"`
	Transactions []Transaction
}
