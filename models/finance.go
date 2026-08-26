package models

import "gorm.io/gorm"

type Finance struct {
	gorm.Model
	UserID         uint                 `json:"user_id" gorm:"index;not null"`
	User           User                 `gorm:"foreignKey:UserID"`
	FinanceTypeID  uint                 `json:"finance_type_id" gorm:"index;not null"`
	FinanceType    FinanceType          `gorm:"foreignKey:FinanceTypeID"`
	Title          *string              `json:"title" gorm:"size:255"`
	Description    *string              `json:"description" gorm:"size:500"`
	Transactions   []Transaction        `json:"transactions"`
	Subcategories  []ExpenseSubcategory `json:"subcategories"`
	Categories     []ExpenseCategory    `json:"categories"`
	IncomeSources  []IncomeSource       `json:"income_sources"`
	SharedFinances []SharedFinance      `gorm:"foreignKey:FinanceID" json:"shared_finances"`
}
