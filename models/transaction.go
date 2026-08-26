package models

import (
	"time"

	"gorm.io/gorm"
)

type Transaction struct {
	gorm.Model
	FinanceID            uint                `json:"finance_id" gorm:"index;not null"`
	Finance              Finance             `gorm:"foreignKey:FinanceID"`
	Description          *string             `json:"description" gorm:"size:500"`
	EntryTypeID          uint                `json:"entry_type_id" gorm:"index;not null"`
	EntryType            EntryType           `gorm:"foreignKey:EntryTypeID"`
	IncomeSourceID       *uint               `json:"income_source_id" gorm:"index"`
	IncomeSource         *IncomeSource       `gorm:"foreignKey:IncomeSourceID"`
	ExpenseCategoryID    *uint               `json:"expense_category_id" gorm:"index"`
	ExpenseCategory      *ExpenseCategory    `gorm:"foreignKey:ExpenseCategoryID"`
	ExpenseSubcategoryID *uint               `json:"expense_subcategory_id" gorm:"index"`
	ExpenseSubcategory   *ExpenseSubcategory `gorm:"foreignKey:ExpenseSubcategoryID"`
	BudgetTypeID         *uint               `json:"budget_type_id" gorm:"index"`
	BudgetType           *BudgetType         `gorm:"foreignKey:BudgetTypeID"`
	UserID               uint                `json:"created_by_user_id" gorm:"index"`
	User                 User                `gorm:"foreignKey:UserID"`
	OccurredAt           time.Time           `json:"occurred_at" gorm:"not null"`
	Amount               float64             `json:"amount" gorm:"not null"`
}
