package main

import (
	"log"
	"pdm-backend/models"
	"pdm-backend/repositories"
)

func main() {

	db := repositories.GetDB()

	err := db.Migrator().DropTable(
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
		log.Fatal("Error resetting the database: ", err)
	}

}
