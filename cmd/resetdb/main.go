package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"os"
	"pdm-backend/internal/config"
	"pdm-backend/models"
	"pdm-backend/repositories"
	"strings"
)

func main() {
	cfg := config.Get()

	if cfg.ENV == "production" {
		log.Fatal("This command should not be run in production environment.")
	}

	dbURL, err := url.Parse(cfg.DATABASE_URL)
	if err != nil {
		log.Fatal("Could not parse DATABASE_URL: ", err)
	}
	dbName := strings.TrimPrefix(dbURL.Path, "/")

	fmt.Printf("This will drop every table in %q on %s.\n", dbName, dbURL.Host)
	fmt.Print("Type the database name to confirm: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		log.Fatal("Could not read confirmation: ", err)
	}

	if strings.TrimSpace(input) != dbName {
		log.Fatal("Confirmation did not match, aborting")
	}

	db := repositories.GetDB()

	err = db.Migrator().DropTable(
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
