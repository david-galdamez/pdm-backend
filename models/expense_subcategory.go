package models

import "gorm.io/gorm"

type ExpenseSubcategory struct {
	gorm.Model
	FinanceID         uint            `json:"finance_id" gorm:"index;not null"`
	Finance           Finance         `gorm:"foreignKey:FinanceID"`
	Name              string          `json:"name" gorm:"not null"`
	ExpenseCategoryID uint            `json:"expense_category_id" gorm:"index;not null"`
	ExpenseCategory   ExpenseCategory `gorm:"foreignKey:ExpenseCategoryID"`
	BudgetTypeID      uint            `json:"budget_type_id" gorm:"index;not null"`
	BudgetType        BudgetType      `gorm:"foreignKey:BudgetTypeID"`
	MonthlyBudget     float64         `json:"monthly_budget" gorm:"not null"`
	UserID            uint            `json:"created_by_user_id" gorm:"index"`
	User              User            `gorm:"foreignKey:UserID"`
	Transactions      []Transaction   `json:"transactions"`
}
