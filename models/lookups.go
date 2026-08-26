package models

import (
	"gorm.io/gorm"
)

type FinanceType struct {
	gorm.Model
	Name     string    `json:"finance_type"`
	Finances []Finance `gorm:"foreignKey:FinanceTypeID" json:"finances"`
}

type SharedFinanceRole struct {
	gorm.Model
	Name string `json:"role"`
}

type BudgetType struct {
	gorm.Model
	Name          string               `json:"budget_type"`
	Subcategories []ExpenseSubcategory `json:"subcategories"`
	Transactions  []Transaction        `gorm:"foreignKey:BudgetTypeID" json:"transactions"`
}

type EntryType struct {
	gorm.Model
	Name         string        `json:"entry_type"`
	Transactions []Transaction `json:"transactions"`
}
