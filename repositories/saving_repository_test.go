package repositories

import (
	"pdm-backend/models"
	"sync"
	"testing"
	"time"
)

// TestCreateOrUpdateSavingGoalIsIdempotent covers the ordinary path: setting a
// goal twice updates the row rather than accumulating rows.
func TestCreateOrUpdateSavingGoalIsIdempotent(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "goal")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	repo := NewSavingRepository(db)

	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	if err := repo.CreateOrUpdateSavingGoal(finance.ID, 100, month, year); err != nil {
		t.Fatalf("first goal: %v", err)
	}

	if err := repo.CreateOrUpdateSavingGoal(finance.ID, 250, month, year); err != nil {
		t.Fatalf("second goal: %v", err)
	}

	var goals []models.MonthlyGoal
	if err := db.Where("finance_id = ?", finance.ID).Find(&goals).Error; err != nil {
		t.Fatalf("reading goals: %v", err)
	}

	if len(goals) != 1 {
		t.Fatalf("expected 1 goal row, got %d", len(goals))
	}

	if goals[0].TargetAmount != 250 {
		t.Errorf("target amount = %v, want 250", goals[0].TargetAmount)
	}
}

// TestCreateOrUpdateSavingGoalConcurrently is the regression test for the
// read-then-write race: several requests for the same period must converge on
// one row instead of inserting competing ones.
func TestCreateOrUpdateSavingGoalConcurrently(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "goalrace")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	repo := NewSavingRepository(db)

	now := time.Now()
	month := int(now.Month())
	year := now.Year()

	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for range 10 {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := repo.CreateOrUpdateSavingGoal(finance.ID, 500, month, year); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("CreateOrUpdateSavingGoal: %v", err)
	}

	var goals []models.MonthlyGoal
	if err := db.Where("finance_id = ?", finance.ID).Find(&goals).Error; err != nil {
		t.Fatalf("reading goals: %v", err)
	}

	if len(goals) != 1 {
		t.Errorf("expected 1 goal row, got %d (concurrent writers each inserted their own)", len(goals))
	}
}

// TestGetSavingsDataComputesCompletion pins the percentage arithmetic and the
// month name lookup that reads off the same row.
func TestGetSavingsDataComputesCompletion(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "savingsdata")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)

	now := time.Now()
	year := now.Year()

	if err := db.Create(&models.MonthlyGoal{FinanceID: finance.ID, Month: 3, Year: year, TargetAmount: 400}).Error; err != nil {
		t.Fatalf("creating goal: %v", err)
	}

	if err := db.Create(&models.MonthlySaving{FinanceID: finance.ID, Month: 3, Year: year, Amount: 100}).Error; err != nil {
		t.Fatalf("creating saving: %v", err)
	}

	// A goal with nothing saved against it must still appear, at 0%.
	if err := db.Create(&models.MonthlyGoal{FinanceID: finance.ID, Month: 4, Year: year, TargetAmount: 200}).Error; err != nil {
		t.Fatalf("creating second goal: %v", err)
	}

	savings, err := NewSavingRepository(db).GetSavingsData(finance.ID, year)
	if err != nil {
		t.Fatalf("GetSavingsData: %v", err)
	}

	if len(savings) != 2 {
		t.Fatalf("expected 2 months, got %d", len(savings))
	}

	if savings[0].MonthName != "March" || savings[0].CompletionRatio != 25 {
		t.Errorf("March = %+v, want March at 25%%", savings[0])
	}

	if savings[1].MonthName != "April" || savings[1].CompletionRatio != 0 {
		t.Errorf("April = %+v, want April at 0%%", savings[1])
	}
}

// TestMonthNameToleratesAnOutOfRangeMonth covers the bounds check: indexing
// monthNames directly panicked the whole request on a stray row.
func TestMonthNameToleratesAnOutOfRangeMonth(t *testing.T) {
	for _, month := range []int{-1, 0, 13, 99} {
		if got := monthName(month); got != "" {
			t.Errorf("monthName(%d) = %q, want an empty string", month, got)
		}
	}

	if got := monthName(1); got != "January" {
		t.Errorf("monthName(1) = %q, want January", got)
	}

	if got := monthName(12); got != "December" {
		t.Errorf("monthName(12) = %q, want December", got)
	}
}
