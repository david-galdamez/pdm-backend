package repositories

import (
	"pdm-backend/models"
	"testing"
	"time"
)

func currentMonthWindow() (time.Time, time.Time) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	return start, start.AddDate(0, 1, 0)
}

// TestGetDataSummaryExcludesDeletedSubcategories is the regression test for the
// soft-delete leak: GORM only filters deleted_at on the primary model, so a
// deleted subcategory reached through a LEFT JOIN kept contributing its budget
// to the dashboard while GetExpenseSummary had already stopped counting it.
func TestGetDataSummaryExcludesDeletedSubcategories(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "dashboard")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Food")

	createSubcategory(t, db, category, user, "Kept", 100)
	removed := createSubcategory(t, db, category, user, "Removed", 250)

	if err := db.Delete(&removed).Error; err != nil {
		t.Fatalf("soft-deleting subcategory: %v", err)
	}

	monthStart, monthEnd := currentMonthWindow()

	results, err := NewFinanceRepository(db).GetDataSummary(monthStart, monthEnd, finance.ID)
	if err != nil {
		t.Fatalf("GetDataSummary: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 category, got %d", len(results))
	}

	if got := results[0].TotalBudget; got != 100 {
		t.Errorf("total budget = %v, want 100 (a soft-deleted subcategory was counted)", got)
	}
}

// TestGetDataSummaryDoesNotFanOut guards the same aggregation hazard the
// category breakdown had: several subcategories and several transactions in
// one category must not multiply each other.
func TestGetDataSummaryDoesNotFanOut(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "fanout2")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Bills")

	first := createSubcategory(t, db, category, user, "Water", 40)
	second := createSubcategory(t, db, category, user, "Power", 60)

	createExpense(t, db, first, user, 10, thisMonth())
	createExpense(t, db, first, user, 5, thisMonth())
	createExpense(t, db, second, user, 20, thisMonth())

	monthStart, monthEnd := currentMonthWindow()

	results, err := NewFinanceRepository(db).GetDataSummary(monthStart, monthEnd, finance.ID)
	if err != nil {
		t.Fatalf("GetDataSummary: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 category, got %d", len(results))
	}

	if got := results[0].TotalBudget; got != 100 {
		t.Errorf("total budget = %v, want 100 (40 + 60)", got)
	}

	if got := results[0].Spent; got != 35 {
		t.Errorf("spent = %v, want 35 (10 + 5 + 20)", got)
	}

	if got := results[0].Difference; got != 65 {
		t.Errorf("difference = %v, want 65", got)
	}
}

// TestGetDataSummaryUsesTheGoalForSavings keeps the savings special case
// working after the per-category loop was collapsed into one query.
func TestGetDataSummaryUsesTheGoalForSavings(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "dashsavings")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, models.SavingsCategoryName)
	createSubcategory(t, db, category, user, models.SavingsCategoryName, 0)

	now := time.Now()

	if err := db.Create(&models.MonthlyGoal{
		FinanceID: finance.ID, Month: int(now.Month()), Year: now.Year(), TargetAmount: 300,
	}).Error; err != nil {
		t.Fatalf("creating goal: %v", err)
	}

	if err := db.Create(&models.MonthlySaving{
		FinanceID: finance.ID, Month: int(now.Month()), Year: now.Year(), Amount: 120,
	}).Error; err != nil {
		t.Fatalf("creating saving: %v", err)
	}

	monthStart, monthEnd := currentMonthWindow()

	results, err := NewFinanceRepository(db).GetDataSummary(monthStart, monthEnd, finance.ID)
	if err != nil {
		t.Fatalf("GetDataSummary: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 category, got %d", len(results))
	}

	if results[0].TotalBudget != 300 || results[0].Spent != 120 || results[0].Difference != 180 {
		t.Errorf("savings row = %+v, want budget 300, spent 120, difference 180", results[0])
	}
}

// TestDashboardAndCategoryBudgetsAgree is the point of the soft-delete sweep:
// the two figures the client shows side by side are produced by different
// queries and must not disagree.
func TestDashboardAndCategoryBudgetsAgree(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "agreement")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Transport")

	createSubcategory(t, db, category, user, "Bus", 30)
	removed := createSubcategory(t, db, category, user, "Taxi", 90)

	if err := db.Delete(&removed).Error; err != nil {
		t.Fatalf("soft-deleting subcategory: %v", err)
	}

	monthStart, monthEnd := currentMonthWindow()

	dashboard, err := NewFinanceRepository(db).GetDataSummary(monthStart, monthEnd, finance.ID)
	if err != nil {
		t.Fatalf("GetDataSummary: %v", err)
	}

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if len(dashboard) != 1 {
		t.Fatalf("expected 1 dashboard category, got %d", len(dashboard))
	}

	if dashboard[0].TotalBudget != breakdown.Budget {
		t.Errorf("dashboard budget %v disagrees with the category breakdown %v", dashboard[0].TotalBudget, breakdown.Budget)
	}
}

// TestGetFinanceSummarySplitsIncomeAndExpense pins the entry-type constants
// that replaced the literal 1 and 2 in the raw SQL.
func TestGetFinanceSummarySplitsIncomeAndExpense(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "summary")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)

	source := models.IncomeSource{FinanceID: finance.ID, Name: "Salary", Amount: 1000, UserID: user.ID}
	if err := db.Create(&source).Error; err != nil {
		t.Fatalf("creating income source: %v", err)
	}

	income := models.Transaction{
		FinanceID: finance.ID, EntryTypeID: models.EntryTypeIncome, IncomeSourceID: &source.ID,
		UserID: user.ID, Amount: 1000, OccurredAt: thisMonth(),
	}
	if err := db.Create(&income).Error; err != nil {
		t.Fatalf("creating income: %v", err)
	}

	category := createCategory(t, db, finance, user, "Food")
	subcategory := createSubcategory(t, db, category, user, "Lunch", 200)
	createExpense(t, db, subcategory, user, 250, thisMonth())

	monthStart, monthEnd := currentMonthWindow()

	summary, err := NewFinanceRepository(db).GetFinanceSummary(finance.ID, monthStart, monthEnd)
	if err != nil {
		t.Fatalf("GetFinanceSummary: %v", err)
	}

	if summary["total_income"] != 1000.0 {
		t.Errorf("total income = %v, want 1000", summary["total_income"])
	}

	if summary["total_expense"] != 250.0 {
		t.Errorf("total expense = %v, want 250", summary["total_expense"])
	}

	if summary["difference"] != 750.0 {
		t.Errorf("difference = %v, want 750", summary["difference"])
	}
}
