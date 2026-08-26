package repositories

import (
	"database/sql"
	"fmt"
	"pdm-backend/models"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// brokenDB hands back a gorm handle whose every query fails, so the cases that
// care about error handling can exercise the failure path without a database.
// It reuses lazyConnector but leaves DryRun off, so the query is really
// attempted and really fails.
func brokenDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sql.OpenDB(lazyConnector{})}),
		&gorm.Config{DisableAutomaticPing: true, Logger: logger.Default.LogMode(logger.Silent)},
	)
	if err != nil {
		t.Fatalf("opening unusable database: %v", err)
	}

	return db
}

// thisMonth is a timestamp safely inside the current month, used wherever a
// case needs a transaction the month-scoped queries will pick up. The 15th
// keeps the date away from month boundaries in any timezone.
func thisMonth() time.Time {
	now := time.Now()

	return time.Date(now.Year(), now.Month(), 15, 12, 0, 0, 0, now.Location())
}

func createUser(t *testing.T, db *gorm.DB, name string) models.User {
	t.Helper()

	user := models.User{
		Name:         name,
		Email:        fmt.Sprintf("%s-%d@example.test", name, time.Now().UnixNano()),
		PasswordHash: "not-a-real-hash",
	}

	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("creating user %s: %v", name, err)
	}

	return user
}

func createFinance(t *testing.T, db *gorm.DB, owner models.User, financeType uint) models.Finance {
	t.Helper()

	finance := models.Finance{UserID: owner.ID, FinanceTypeID: financeType}

	if financeType == models.FinanceTypeShared {
		title := "Shared finance"
		description := "created by a test"
		finance.Title = &title
		finance.Description = &description
	}

	if err := db.Create(&finance).Error; err != nil {
		t.Fatalf("creating finance: %v", err)
	}

	return finance
}

func createCategory(t *testing.T, db *gorm.DB, finance models.Finance, owner models.User, name string) models.ExpenseCategory {
	t.Helper()

	category := models.ExpenseCategory{FinanceID: finance.ID, Name: name, UserID: owner.ID}

	if err := db.Create(&category).Error; err != nil {
		t.Fatalf("creating category %s: %v", name, err)
	}

	return category
}

func createSubcategory(t *testing.T, db *gorm.DB, category models.ExpenseCategory, owner models.User, name string, budget float64) models.ExpenseSubcategory {
	t.Helper()

	subcategory := models.ExpenseSubcategory{
		FinanceID:         category.FinanceID,
		ExpenseCategoryID: category.ID,
		Name:              name,
		BudgetTypeID:      models.BudgetTypeVariable,
		MonthlyBudget:     budget,
		UserID:            owner.ID,
	}

	if err := db.Create(&subcategory).Error; err != nil {
		t.Fatalf("creating subcategory %s: %v", name, err)
	}

	return subcategory
}

func createExpense(t *testing.T, db *gorm.DB, subcategory models.ExpenseSubcategory, owner models.User, amount float64, occurredAt time.Time) models.Transaction {
	t.Helper()

	transaction := models.Transaction{
		FinanceID:            subcategory.FinanceID,
		EntryTypeID:          models.EntryTypeExpense,
		ExpenseCategoryID:    &subcategory.ExpenseCategoryID,
		ExpenseSubcategoryID: &subcategory.ID,
		BudgetTypeID:         &subcategory.BudgetTypeID,
		UserID:               owner.ID,
		Amount:               amount,
		OccurredAt:           occurredAt,
	}

	if err := db.Create(&transaction).Error; err != nil {
		t.Fatalf("creating expense: %v", err)
	}

	return transaction
}
