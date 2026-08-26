package main

import (
	"log"
	"pdm-backend/models"
	"pdm-backend/repositories"

	"gorm.io/gorm"
)

func main() {

	db := repositories.GetDB()

	err := db.AutoMigrate(
		&models.FinanceType{},
		&models.BudgetType{},
		&models.EntryType{},
		&models.IncomeSource{},
		&models.SharedFinanceRole{},
		&models.User{},
		&models.Finance{},
		&models.SharedFinance{},
		&models.ExpenseCategory{},
		&models.ExpenseSubcategory{},
		&models.Transaction{},
		&models.MonthlyGoal{},
		&models.MonthlySaving{},
		&models.Invitation{},
	)
	if err != nil {
		log.Fatal("An error occurred while running the migrations: ", err)
	}

	SeedData()

	log.Print("Migrations completed successfully")
}

func SeedData() {

	db := repositories.GetDB()

	financeTypes := []models.FinanceType{
		{Name: "personal"},
		{Name: "shared"},
	}
	for _, financeType := range financeTypes {
		var existing models.FinanceType

		if err := db.Where("name = ?", financeType.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&financeType)
			}
		}
	}

	roles := []models.SharedFinanceRole{
		{Name: "admin"},
		{Name: "collaborator"},
	}
	for _, role := range roles {
		var existing models.SharedFinanceRole

		if err := db.Where("name = ?", role.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&role)
			}
		}
	}

	budgetTypes := []models.BudgetType{
		{Name: "Variable expenses"},
		{Name: "Fixed expenses"},
		{Name: "Provisional expenses"},
	}
	for _, budgetType := range budgetTypes {
		var existing models.BudgetType

		if err := db.Where("name = ?", budgetType.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&budgetType)
			}
		}
	}

	entryTypes := []models.EntryType{
		{Name: "Income"},
		{Name: "Expense"},
	}
	for _, entryType := range entryTypes {
		var existing models.EntryType

		if err := db.Where("name = ?", entryType.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&entryType)
			}
		}
	}

}
