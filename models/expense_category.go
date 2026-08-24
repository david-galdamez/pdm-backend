package models

import "gorm.io/gorm"

type ExpenseCategory struct {
	gorm.Model
	FinanceID     uint                 `json:"finance_id" gorm:"index;not null"`
	Finance       Finance              `gorm:"foreignKey:FinanceID"`
	Name          string               `json:"name" gorm:"not null"`
	UserID        uint                 `json:"created_by_user_id" gorm:"index"`
	User          User                 `gorm:"foreignKey:UserID"`
	Transactions  []Transaction        `json:"transactions"`
	Subcategories []ExpenseSubcategory `json:"subcategories"`
}
