package repositories

import (
	"errors"
	"pdm-backend/models"
	"sync"
	"testing"

	"gorm.io/gorm"
)

// TestCreateOrUpdateSavingAccumulatesConcurrently is the regression test for
// the lost update: the old implementation read the row into Go, added to it,
// and wrote it back, so two contributions landing together kept only one.
func TestCreateOrUpdateSavingAccumulatesConcurrently(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "concurrent")
	finance := createFinance(t, db, user, models.FinanceTypeShared)
	repo := NewTransactionRepository(db)

	const contributions = 20
	const each = 5.0

	var wg sync.WaitGroup
	errs := make(chan error, contributions)

	for range contributions {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := repo.CreateOrUpdateSaving(finance.ID, each, thisMonth()); err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Fatalf("CreateOrUpdateSaving: %v", err)
	}

	var rows []models.MonthlySaving
	if err := db.Where("finance_id = ?", finance.ID).Find(&rows).Error; err != nil {
		t.Fatalf("reading savings: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected exactly 1 monthly saving row, got %d (concurrent inserts were not deduplicated)", len(rows))
	}

	if want := contributions * each; rows[0].Amount != want {
		t.Errorf("accumulated %v, want %v (%d contributions were lost)", rows[0].Amount, want, int((want-rows[0].Amount)/each))
	}
}

// TestCreateTransactionWithSavingIsAtomic checks the two writes travel
// together: a transaction that fails to persist must not leave its amount
// counted in the month's savings.
func TestCreateTransactionWithSavingIsAtomic(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "atomic")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	repo := NewTransactionRepository(db)

	// entry_type_id has a foreign key to the lookup table, so an unknown one
	// fails the insert the same way any other write failure would.
	doomed := models.Transaction{
		FinanceID:   finance.ID,
		EntryTypeID: 9999,
		UserID:      user.ID,
		Amount:      50,
		OccurredAt:  thisMonth(),
	}

	if err := repo.CreateTransactionWithSaving(&doomed, true); err == nil {
		t.Fatal("expected the transaction insert to fail")
	}

	var savings int64
	if err := db.Model(&models.MonthlySaving{}).Where("finance_id = ?", finance.ID).Count(&savings).Error; err != nil {
		t.Fatalf("counting savings: %v", err)
	}

	if savings != 0 {
		t.Errorf("found %d saving rows after a failed transaction; the two writes are not atomic", savings)
	}
}

// TestCreateTransactionWithSavingRollsUpTheSaving is the positive half: a
// savings transaction still accumulates.
func TestCreateTransactionWithSavingRollsUpTheSaving(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "rollup")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)
	category := createCategory(t, db, finance, user, models.SavingsCategoryName)
	subcategory := createSubcategory(t, db, category, user, models.SavingsCategoryName, 0)

	transaction := models.Transaction{
		FinanceID:            finance.ID,
		EntryTypeID:          models.EntryTypeExpense,
		ExpenseCategoryID:    &category.ID,
		ExpenseSubcategoryID: &subcategory.ID,
		UserID:               user.ID,
		Amount:               125,
		OccurredAt:           thisMonth(),
	}

	if err := NewTransactionRepository(db).CreateTransactionWithSaving(&transaction, true); err != nil {
		t.Fatalf("CreateTransactionWithSaving: %v", err)
	}

	var saving models.MonthlySaving
	if err := db.Where("finance_id = ?", finance.ID).First(&saving).Error; err != nil {
		t.Fatalf("reading saving: %v", err)
	}

	if saving.Amount != 125 {
		t.Errorf("saving amount = %v, want 125", saving.Amount)
	}
}

// TestGetIdsRejectsAnotherFinancesSubcategory covers the scoping added along
// with the not-found error: the movement id on a create request is
// client-supplied, and resolving it unscoped let one finance file expenses
// against another finance's subcategory.
func TestGetIdsRejectsAnotherFinancesSubcategory(t *testing.T) {
	db := requireDB(t)

	owner := createUser(t, db, "owner")
	ownerFinance := createFinance(t, db, owner, models.FinanceTypePersonal)
	ownerCategory := createCategory(t, db, ownerFinance, owner, "Private")
	ownerSubcategory := createSubcategory(t, db, ownerCategory, owner, "Private spending", 100)

	stranger := createUser(t, db, "stranger")
	strangerFinance := createFinance(t, db, stranger, models.FinanceTypePersonal)

	repo := NewTransactionRepository(db)

	if _, err := repo.GetIds(ownerSubcategory.ID, strangerFinance.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetIds across finances = %v, want gorm.ErrRecordNotFound", err)
	}

	identifiers, err := repo.GetIds(ownerSubcategory.ID, ownerFinance.ID)
	if err != nil {
		t.Fatalf("GetIds within the finance: %v", err)
	}

	if identifiers.CategoryID != ownerCategory.ID {
		t.Errorf("category id = %d, want %d", identifiers.CategoryID, ownerCategory.ID)
	}
}

// TestGetIdsReportsAMissingSubcategory pins the silent-zero fix: the old
// version returned identifiers full of zeroes and a nil error, which the
// controller then wrote onto the transaction as category id 0.
func TestGetIdsReportsAMissingSubcategory(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "missing")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)

	identifiers, err := NewTransactionRepository(db).GetIds(4242, finance.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetIds on a missing subcategory = (%v, %v), want gorm.ErrRecordNotFound", identifiers, err)
	}
}

// TestGetSavingSubcategoryReportsAMissingRow is the same silent-zero fix on
// the savings lookup, which fed a subcategory id straight into a JWT.
func TestGetSavingSubcategoryReportsAMissingRow(t *testing.T) {
	db := requireDB(t)

	user := createUser(t, db, "nosavings")
	finance := createFinance(t, db, user, models.FinanceTypePersonal)

	if _, err := NewTransactionRepository(db).GetSavingSubcategory(finance.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("GetSavingSubcategory with no savings subcategory = %v, want gorm.ErrRecordNotFound", err)
	}
}

// TestGetTransactionByIdReportsQueryErrors covers the check ordering: a failed
// query also reports zero rows, and testing RowsAffected first turned every
// database error into a 404.
func TestGetTransactionByIdReportsQueryErrors(t *testing.T) {
	db := brokenDB(t)

	id := uint(1)

	_, err := NewTransactionRepository(db).GetTransactionById(&id, 1)
	if err == nil {
		t.Fatal("expected an error from an unusable database")
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Error("a failed query was reported as gorm.ErrRecordNotFound; the caller will answer 404 instead of 500")
	}
}
