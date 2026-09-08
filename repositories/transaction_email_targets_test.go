package repositories

import (
	"errors"
	"testing"

	"pdm-backend/models"

	"gorm.io/gorm"
)

// emailsOf reduces the recipient list to addresses so a case can compare it
// without caring about ordering fields.
func emailsOf(targets *TransactionEmailTargets) map[string]bool {
	found := make(map[string]bool)

	for _, recipient := range targets.Recipients {
		found[recipient.Email] = true
	}

	return found
}

func TestGetTransactionEmailTargetsExcludesTheActor(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "admin")
	member := createUser(t, db, "member")

	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createMembership(t, db, finance, member, models.RoleCollaborator, true)

	repo := NewSharedFinanceRepository(db)

	targets, err := repo.GetTransactionEmailTargets(finance.ID, admin.ID)
	if err != nil {
		t.Fatalf("resolving targets: %v", err)
	}

	// Nobody wants an email about the transaction they just typed in.
	if len(targets.Recipients) != 1 {
		t.Fatalf("got %d recipients, want 1: %+v", len(targets.Recipients), targets.Recipients)
	}

	if targets.Recipients[0].UserID != member.ID {
		t.Errorf("recipient = %d, want the other member %d", targets.Recipients[0].UserID, member.ID)
	}

	if targets.Recipients[0].Email != member.Email || targets.Recipients[0].Name != member.Name {
		t.Errorf("recipient = %+v, want %s <%s>", targets.Recipients[0], member.Name, member.Email)
	}

	if targets.ActorName != admin.Name {
		t.Errorf("ActorName = %q, want %q", targets.ActorName, admin.Name)
	}

	if finance.Title != nil && targets.FinanceName != *finance.Title {
		t.Errorf("FinanceName = %q, want %q", targets.FinanceName, *finance.Title)
	}
}

// A member who left keeps their row with active = false. Mailing them about a
// finance they are no longer in is a leak, not just noise.
func TestGetTransactionEmailTargetsSkipsInactiveMembers(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "admin")
	left := createUser(t, db, "left")
	stayed := createUser(t, db, "stayed")

	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createMembership(t, db, finance, left, models.RoleCollaborator, false)
	createMembership(t, db, finance, stayed, models.RoleCollaborator, true)

	targets, err := NewSharedFinanceRepository(db).GetTransactionEmailTargets(finance.ID, admin.ID)
	if err != nil {
		t.Fatalf("resolving targets: %v", err)
	}

	found := emailsOf(targets)

	if found[left.Email] {
		t.Errorf("the inactive member %s is still a recipient", left.Email)
	}

	if !found[stayed.Email] {
		t.Errorf("the active member %s is missing from %v", stayed.Email, found)
	}
}

// GORM only applies deleted_at IS NULL to the primary model, so the join to
// users has to spell the filter out or a soft-deleted account keeps receiving
// mail.
func TestGetTransactionEmailTargetsSkipsSoftDeletedUsers(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "admin")
	removed := createUser(t, db, "removed")

	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createMembership(t, db, finance, removed, models.RoleCollaborator, true)

	if err := db.Delete(&models.User{}, removed.ID).Error; err != nil {
		t.Fatalf("soft-deleting the user: %v", err)
	}

	targets, err := NewSharedFinanceRepository(db).GetTransactionEmailTargets(finance.ID, admin.ID)
	if err != nil {
		t.Fatalf("resolving targets: %v", err)
	}

	if emailsOf(targets)[removed.Email] {
		t.Errorf("the soft-deleted user %s is still a recipient", removed.Email)
	}
}

// A finance whose only other member has gone yields an empty list and a nil
// error; the worker turns that into an ack, not a dead letter.
func TestGetTransactionEmailTargetsWithNoOtherMembers(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "admin")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)

	targets, err := NewSharedFinanceRepository(db).GetTransactionEmailTargets(finance.ID, admin.ID)
	if err != nil {
		t.Fatalf("resolving targets: %v", err)
	}

	if len(targets.Recipients) != 0 {
		t.Errorf("got %d recipients, want 0: %+v", len(targets.Recipients), targets.Recipients)
	}
}

// Scan fills a struct with zero values and reports no error when nothing
// matched, which would hand the worker a finance called "" instead of telling
// it the finance is gone.
func TestGetTransactionEmailTargetsMissingFinance(t *testing.T) {
	db := requireDB(t)

	_, err := NewSharedFinanceRepository(db).GetTransactionEmailTargets(987654, 1)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

// A failed query also reports zero rows, so the error has to be checked before
// the row count or a broken database looks like a deleted finance.
func TestGetTransactionEmailTargetsReportsQueryFailures(t *testing.T) {
	_, err := NewSharedFinanceRepository(brokenDB(t)).GetTransactionEmailTargets(1, 1)
	if err == nil {
		t.Fatal("GetTransactionEmailTargets reported success against an unusable database")
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Error("a query failure was reported as a missing finance")
	}
}
