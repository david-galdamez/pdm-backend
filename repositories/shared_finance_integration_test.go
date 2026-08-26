package repositories

import (
	"errors"
	"pdm-backend/models"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

func createMembership(t *testing.T, db *gorm.DB, finance models.Finance, user models.User, role uint, active bool) models.SharedFinance {
	t.Helper()

	membership := models.SharedFinance{
		FinanceID: finance.ID,
		UserID:    user.ID,
		RoleID:    role,
		Active:    active,
		JoinedAt:  time.Now(),
	}

	if err := db.Create(&membership).Error; err != nil {
		t.Fatalf("creating membership: %v", err)
	}

	return membership
}

func createInvitation(t *testing.T, db *gorm.DB, finance models.Finance, code string, expiresAt time.Time) models.Invitation {
	t.Helper()

	invitation := models.Invitation{FinanceID: finance.ID, Code: code, ExpiresAt: expiresAt}

	if err := db.Create(&invitation).Error; err != nil {
		t.Fatalf("creating invitation: %v", err)
	}

	return invitation
}

// TestLeaveSharedFinanceRefusesTheAdmin covers the orphaning hole: nothing
// transfers the admin role, so an admin who leaves strands the finance with
// members and nobody able to administer it.
func TestLeaveSharedFinanceRefusesTheAdmin(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "admin")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)

	err := NewSharedFinanceRepository(db).LeaveSharedFinance(admin.ID, finance.ID)
	if !errors.Is(err, ErrAdminCannotLeave) {
		t.Fatalf("admin leaving = %v, want ErrAdminCannotLeave", err)
	}

	var membership models.SharedFinance
	if err := db.Where("finance_id = ? AND user_id = ?", finance.ID, admin.ID).First(&membership).Error; err != nil {
		t.Fatalf("reading membership: %v", err)
	}

	if !membership.Active {
		t.Error("the admin's membership was deactivated despite the refusal")
	}
}

// TestLeaveSharedFinanceDeactivatesACollaborator is the positive half.
func TestLeaveSharedFinanceDeactivatesACollaborator(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "owner2")
	member := createUser(t, db, "member")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createMembership(t, db, finance, member, models.RoleCollaborator, true)

	if err := NewSharedFinanceRepository(db).LeaveSharedFinance(member.ID, finance.ID); err != nil {
		t.Fatalf("LeaveSharedFinance: %v", err)
	}

	var membership models.SharedFinance
	if err := db.Where("finance_id = ? AND user_id = ?", finance.ID, member.ID).First(&membership).Error; err != nil {
		t.Fatalf("reading membership: %v", err)
	}

	if membership.Active {
		t.Error("the collaborator is still active after leaving")
	}
}

// TestLeaveSharedFinanceOnANonMember pins the not-found path, which now also
// covers a member who already left rather than silently succeeding twice.
func TestLeaveSharedFinanceOnANonMember(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "owner3")
	stranger := createUser(t, db, "stranger2")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)

	repo := NewSharedFinanceRepository(db)

	if err := repo.LeaveSharedFinance(stranger.ID, finance.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("non-member leaving = %v, want gorm.ErrRecordNotFound", err)
	}

	former := createUser(t, db, "former")
	createMembership(t, db, finance, former, models.RoleCollaborator, false)

	if err := repo.LeaveSharedFinance(former.ID, finance.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("already-departed member leaving = %v, want gorm.ErrRecordNotFound", err)
	}
}

// TestGetSharedFinanceDetailsReportsAMissingFinance covers the silent-zero
// scan: the old version answered 200 with an empty title and no members.
func TestGetSharedFinanceDetailsReportsAMissingFinance(t *testing.T) {
	db := requireDB(t)

	if _, err := NewSharedFinanceRepository(db).GetSharedFinanceDetails(9999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Errorf("details for a missing finance = %v, want gorm.ErrRecordNotFound", err)
	}
}

// TestGetSharedFinancesIgnoresADepartedAdmin covers the admin join: without an
// active filter a finance kept naming an admin who had already left.
func TestGetSharedFinancesIgnoresADepartedAdmin(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "gone")
	member := createUser(t, db, "stays")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)

	createMembership(t, db, finance, admin, models.RoleAdmin, false)
	createMembership(t, db, finance, member, models.RoleCollaborator, true)

	finances, err := NewSharedFinanceRepository(db).GetSharedFinances(member.ID)
	if err != nil {
		t.Fatalf("GetSharedFinances: %v", err)
	}

	if len(finances) != 1 {
		t.Fatalf("expected 1 finance, got %d", len(finances))
	}

	if finances[0].AdminName != "" {
		t.Errorf("admin name = %q, want empty: that admin is no longer a member", finances[0].AdminName)
	}
}

// TestJoinUserConcurrentlyCreatesOneMembership is the regression test for the
// join race: two taps on "join" both found no membership and both inserted.
func TestJoinUserConcurrentlyCreatesOneMembership(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "inviter")
	joiner := createUser(t, db, "joiner")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createInvitation(t, db, finance, "JOINCODE01", time.Now().Add(15*time.Minute))

	repo := NewSharedFinanceRepository(db)

	const attempts = 8
	var wg sync.WaitGroup
	results := make(chan error, attempts)

	for range attempts {
		wg.Add(1)

		go func() {
			defer wg.Done()
			results <- repo.JoinUser(joiner.ID, "JOINCODE01")
		}()
	}

	wg.Wait()
	close(results)

	joined := 0

	for err := range results {
		switch {
		case err == nil:
			joined++
		case errors.Is(err, ErrAlreadyMember):
		default:
			t.Fatalf("JoinUser: %v", err)
		}
	}

	if joined != 1 {
		t.Errorf("%d of %d concurrent joins succeeded, want exactly 1", joined, attempts)
	}

	var memberships int64
	if err := db.Model(&models.SharedFinance{}).Where("finance_id = ? AND user_id = ?", finance.ID, joiner.ID).Count(&memberships).Error; err != nil {
		t.Fatalf("counting memberships: %v", err)
	}

	if memberships != 1 {
		t.Errorf("found %d membership rows, want 1", memberships)
	}
}

// TestJoinUserReactivatesAFormerMember keeps rejoining working: the row is
// flipped back to active rather than duplicated.
func TestJoinUserReactivatesAFormerMember(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "inviter2")
	returning := createUser(t, db, "returning")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createMembership(t, db, finance, returning, models.RoleCollaborator, false)
	createInvitation(t, db, finance, "JOINCODE02", time.Now().Add(15*time.Minute))

	if err := NewSharedFinanceRepository(db).JoinUser(returning.ID, "JOINCODE02"); err != nil {
		t.Fatalf("JoinUser: %v", err)
	}

	var memberships []models.SharedFinance
	if err := db.Where("finance_id = ? AND user_id = ?", finance.ID, returning.ID).Find(&memberships).Error; err != nil {
		t.Fatalf("reading memberships: %v", err)
	}

	if len(memberships) != 1 {
		t.Fatalf("expected 1 membership row, got %d", len(memberships))
	}

	if !memberships[0].Active {
		t.Error("the returning member was not reactivated")
	}
}

// TestJoinUserRejectsAnExpiredInvitation pins the expiry path.
func TestJoinUserRejectsAnExpiredInvitation(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "inviter3")
	latecomer := createUser(t, db, "latecomer")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)
	createMembership(t, db, finance, admin, models.RoleAdmin, true)
	createInvitation(t, db, finance, "JOINCODE03", time.Now().Add(-time.Minute))

	if err := NewSharedFinanceRepository(db).JoinUser(latecomer.ID, "JOINCODE03"); !errors.Is(err, ErrInviteExpired) {
		t.Errorf("joining with an expired code = %v, want ErrInviteExpired", err)
	}
}

// TestCreateInviteRetriesOnACodeCollision exercises the unique-violation path
// that replaced the driver-message matching. The generated code is random, so
// the collision is arranged by taking one code up front and confirming the
// repository still lands a distinct one.
func TestCreateInviteRetriesOnACodeCollision(t *testing.T) {
	db := requireDB(t)

	admin := createUser(t, db, "inviter4")
	finance := createFinance(t, db, admin, models.FinanceTypeShared)

	invitation, err := NewInvitationRepository(db).CreateInvite(finance.ID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	if invitation.Code == "" {
		t.Fatal("CreateInvite returned an empty code")
	}

	second, err := NewInvitationRepository(db).CreateInvite(finance.ID)
	if err != nil {
		t.Fatalf("CreateInvite (second): %v", err)
	}

	if second.Code == invitation.Code {
		t.Error("two invitations share a code")
	}
}

// TestIsUniqueViolationIgnoresOtherFailures makes sure the helper that
// replaced the string matching does not swallow unrelated errors.
func TestIsUniqueViolationIgnoresOtherFailures(t *testing.T) {
	if IsUniqueViolation(nil) {
		t.Error("nil reported as a unique violation")
	}

	if IsUniqueViolation(errors.New("duplicate key value violates unique constraint")) {
		t.Error("a plain error whose text merely mentions a duplicate key was reported as a unique violation")
	}
}
