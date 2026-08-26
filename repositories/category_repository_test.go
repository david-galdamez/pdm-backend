package repositories

import (
	"pdm-backend/models"
	"testing"
	"time"
)

// TestGetCategoriesDataBudgetIsNotMultipliedByTransactionCount is the
// regression test for the join fan-out: aggregating over a LEFT JOIN on
// transactions emitted one row per transaction, so SUM(monthly_budget)
// counted the budget once per transaction filed against it.
func TestGetCategoriesDataBudgetIsNotMultipliedByTransactionCount(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "fanout")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Groceries")
	subcategory := createSubcategory(t, db, category, user, "Supermarket", 100)

	// Three transactions against one subcategory: enough to turn a 100 budget
	// into 300 if the query fans out.
	for _, amount := range []float64{10, 20, 30} {
		createExpense(t, db, subcategory, user, amount, thisMonth())
	}

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if len(breakdown.Subcategories) != 1 {
		t.Fatalf("expected 1 subcategory, got %d", len(breakdown.Subcategories))
	}

	if got := breakdown.Subcategories[0].Budget; got != 100 {
		t.Errorf("subcategory budget = %v, want 100 (multiplied by the transaction count?)", got)
	}

	if got := breakdown.Subcategories[0].Spent; got != 60 {
		t.Errorf("subcategory spent = %v, want 60", got)
	}

	if got := breakdown.Budget; got != 100 {
		t.Errorf("category budget = %v, want 100", got)
	}

	if got := breakdown.Spent; got != 60 {
		t.Errorf("category spent = %v, want 60", got)
	}

	if got := breakdown.Difference; got != 40 {
		t.Errorf("category difference = %v, want 40", got)
	}
}

// TestGetCategoriesDataTotalsCoverEverySubcategory is the regression test for
// the totals query: it grouped per subcategory but scanned into a single
// struct, so the "category total" was whichever row came back first.
func TestGetCategoriesDataTotalsCoverEverySubcategory(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "totals")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Home")

	first := createSubcategory(t, db, category, user, "Rent", 500)
	second := createSubcategory(t, db, category, user, "Utilities", 200)

	createExpense(t, db, first, user, 450, thisMonth())
	createExpense(t, db, second, user, 75, thisMonth())

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if len(breakdown.Subcategories) != 2 {
		t.Fatalf("expected 2 subcategories, got %d", len(breakdown.Subcategories))
	}

	if got := breakdown.Budget; got != 700 {
		t.Errorf("category budget = %v, want 700 (500 + 200)", got)
	}

	if got := breakdown.Spent; got != 525 {
		t.Errorf("category spent = %v, want 525 (450 + 75)", got)
	}

	if got := breakdown.Difference; got != 175 {
		t.Errorf("category difference = %v, want 175", got)
	}
}

// TestGetCategoriesDataSpendIsScopedToTheCurrentMonth guards the other half of
// the join fix: the old SUM had no date filter, so a category's "spent" was
// its all-time total against a budget that resets monthly.
func TestGetCategoriesDataSpendIsScopedToTheCurrentMonth(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "window")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Travel")
	subcategory := createSubcategory(t, db, category, user, "Fuel", 300)

	createExpense(t, db, subcategory, user, 40, thisMonth())
	createExpense(t, db, subcategory, user, 999, thisMonth().AddDate(0, -2, 0))

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if got := breakdown.Spent; got != 40 {
		t.Errorf("category spent = %v, want 40 (a two-month-old expense leaked in?)", got)
	}
}

// TestGetCategoriesDataIgnoresDeletedTransactions covers the soft-delete leak:
// a transaction reached through a join is not filtered by GORM, only the
// primary model is.
func TestGetCategoriesDataIgnoresDeletedTransactions(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "deleted")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Leisure")
	subcategory := createSubcategory(t, db, category, user, "Cinema", 50)

	createExpense(t, db, subcategory, user, 15, thisMonth())
	removed := createExpense(t, db, subcategory, user, 25, thisMonth())

	if err := db.Delete(&removed).Error; err != nil {
		t.Fatalf("soft-deleting transaction: %v", err)
	}

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if got := breakdown.Spent; got != 15 {
		t.Errorf("category spent = %v, want 15 (a soft-deleted transaction was counted)", got)
	}
}

// TestGetCategoriesDataUsesTheGoalForTheSavingsCategory keeps the special case
// intact: savings carries no subcategory budget, its budget is the month goal.
func TestGetCategoriesDataUsesTheGoalForTheSavingsCategory(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "savings")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, models.SavingsCategoryName)
	createSubcategory(t, db, category, user, models.SavingsCategoryName, 0)

	now := time.Now()
	goal := models.MonthlyGoal{
		FinanceID:    finance.ID,
		Month:        int(now.Month()),
		Year:         now.Year(),
		TargetAmount: 400,
	}

	if err := db.Create(&goal).Error; err != nil {
		t.Fatalf("creating goal: %v", err)
	}

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if got := breakdown.Budget; got != 400 {
		t.Errorf("savings budget = %v, want the month goal of 400", got)
	}
}

// TestGetCategoriesDataOnAnEmptyCategory pins the empty case: an empty list,
// never a nil the handler would render as JSON null.
func TestGetCategoriesDataOnAnEmptyCategory(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "empty")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, "Unused")

	breakdown, err := NewCategoryRepository(db).GetCategoriesData(finance.ID, &category.ID)
	if err != nil {
		t.Fatalf("GetCategoriesData: %v", err)
	}

	if breakdown.Subcategories == nil {
		t.Error("subcategories is nil; it must marshal as [] rather than null")
	}

	if breakdown.Budget != 0 || breakdown.Spent != 0 || breakdown.Difference != 0 {
		t.Errorf("expected zeroed totals, got %+v", breakdown)
	}
}
