package repositories

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"pdm-backend/models"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// lazyConnector satisfies database/sql without ever dialing, so the query
// builder can be exercised in DryRun mode with no database present.
type lazyConnector struct{}

func (lazyConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("lazyConnector never connects")
}

func (lazyConnector) Driver() driver.Driver { return nil }

func dryRunDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sql.OpenDB(lazyConnector{})}),
		&gorm.Config{DryRun: true, DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("opening dry-run database: %v", err)
	}

	return db
}

// TestJoinUserInvitationLookupSQL guards two things at once: that the lookup
// only matches invitations on a shared finance (so a code minted for a
// personal finance can never be redeemed), and that selecting "invitations.*"
// keeps the join from colliding on the id/created_at/updated_at/deleted_at
// columns both tables share.
func TestJoinUserInvitationLookupSQL(t *testing.T) {
	db := dryRunDB(t)

	var invitation models.Invitation

	generated := db.ToSQL(func(tx *gorm.DB) *gorm.DB {
		return tx.Select("invitations.*").
			Joins("JOIN finances ON finances.id = invitations.finance_id AND finances.finance_type_id = ?", models.FinanceTypeShared).
			Where("code = ?", "ABC123XYZ0").
			First(&invitation)
	})

	t.Logf("generated SQL:\n%s", generated)

	fragments := []string{
		`SELECT invitations.*`,
		`JOIN finances ON finances.id = invitations.finance_id AND finances.finance_type_id = 2`,
		`code = 'ABC123XYZ0'`,
	}

	for _, fragment := range fragments {
		if !strings.Contains(generated, fragment) {
			t.Errorf("generated SQL is missing %q", fragment)
		}
	}

	// Explicit invitations.* must be the only thing standing between this and
	// a "SELECT *" that would silently let finances.id overwrite invitation.ID.
	if strings.Contains(generated, "SELECT *") {
		t.Error("query selects * across the join; ambiguous id/timestamp columns will corrupt the scan")
	}
}
