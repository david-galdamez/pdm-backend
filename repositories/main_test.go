package repositories

import (
	"fmt"
	"os"
	"pdm-backend/models"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The repositories suite provisions its own database rather than sharing the
// one routes/authz_integration_test.go uses: package test binaries run
// concurrently, and both suites truncate tables between cases.
const repoTestDatabase = "finance_app_repo_test"

// testDB is nil when no local Postgres answered, which turns every integration
// case in this package into a skip instead of a failure. The dry-run cases
// alongside them still run everywhere.
var testDB *gorm.DB

func TestMain(m *testing.M) {
	if db, err := setupTestDatabase(); err != nil {
		fmt.Fprintf(os.Stderr, "repositories: skipping integration cases: %v\n", err)
	} else {
		testDB = db
	}

	os.Exit(m.Run())
}

// baseDSN points at the Postgres server without naming a database. Override it
// with TEST_POSTGRES_DSN when the local server is not the one the rest of the
// test suite assumes.
func baseDSN() string {
	if dsn := os.Getenv("TEST_POSTGRES_DSN"); dsn != "" {
		return dsn
	}

	return "postgres://postgres:analissa@localhost:5432/%s?sslmode=disable"
}

func setupTestDatabase() (*gorm.DB, error) {
	silent := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}

	// Connect to the maintenance database first so the suite can create its
	// own on a machine that has never run it.
	admin, err := gorm.Open(postgres.Open(fmt.Sprintf(baseDSN(), "postgres")), silent)
	if err != nil {
		return nil, fmt.Errorf("no local postgres: %w", err)
	}

	var exists bool
	if err := admin.Raw("SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = ?)", repoTestDatabase).Scan(&exists).Error; err != nil {
		return nil, fmt.Errorf("checking for %s: %w", repoTestDatabase, err)
	}

	if !exists {
		if err := admin.Exec("CREATE DATABASE " + repoTestDatabase).Error; err != nil {
			return nil, fmt.Errorf("creating %s: %w", repoTestDatabase, err)
		}
	}

	if sqlDB, err := admin.DB(); err == nil {
		sqlDB.Close()
	}

	db, err := gorm.Open(postgres.Open(fmt.Sprintf(baseDSN(), repoTestDatabase)), silent)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", repoTestDatabase, err)
	}

	if err := db.AutoMigrate(
		&models.FinanceType{},
		&models.BudgetType{},
		&models.EntryType{},
		&models.SharedFinanceRole{},
		&models.User{},
		&models.Finance{},
		&models.SharedFinance{},
		&models.IncomeSource{},
		&models.ExpenseCategory{},
		&models.ExpenseSubcategory{},
		&models.Transaction{},
		&models.MonthlyGoal{},
		&models.MonthlySaving{},
		&models.Invitation{},
	); err != nil {
		return nil, fmt.Errorf("migrating %s: %w", repoTestDatabase, err)
	}

	seedTestLookups(db)

	return db, nil
}

// seedTestLookups mirrors cmd/migrations so the generated ids line up with
// models/constants.go on a fresh database.
func seedTestLookups(db *gorm.DB) {
	insertIfMissing := func(model any, name string) {
		if db.Where("name = ?", name).First(model).Error == gorm.ErrRecordNotFound {
			db.Create(model)
		}
	}

	insertIfMissing(&models.FinanceType{Name: "personal"}, "personal")
	insertIfMissing(&models.FinanceType{Name: "shared"}, "shared")
	insertIfMissing(&models.SharedFinanceRole{Name: "admin"}, "admin")
	insertIfMissing(&models.SharedFinanceRole{Name: "collaborator"}, "collaborator")
	insertIfMissing(&models.BudgetType{Name: "Variable expenses"}, "Variable expenses")
	insertIfMissing(&models.BudgetType{Name: "Fixed expenses"}, "Fixed expenses")
	insertIfMissing(&models.BudgetType{Name: "Provisional expenses"}, "Provisional expenses")
	insertIfMissing(&models.EntryType{Name: "Income"}, "Income")
	insertIfMissing(&models.EntryType{Name: "Expense"}, "Expense")
}

// requireDB hands back a clean database, or skips the case when none is
// reachable.
func requireDB(t *testing.T) *gorm.DB {
	t.Helper()

	if testDB == nil {
		t.Skip("no local postgres; set TEST_POSTGRES_DSN to run the integration cases")
	}

	tables := []string{
		"invitations", "monthly_savings", "monthly_goals", "transactions",
		"shared_finances", "expense_subcategories", "expense_categories",
		"income_sources", "finances", "users",
	}

	for _, table := range tables {
		if err := testDB.Exec("TRUNCATE TABLE " + table + " RESTART IDENTITY CASCADE").Error; err != nil {
			t.Fatalf("truncating %s: %v", table, err)
		}
	}

	return testDB
}
