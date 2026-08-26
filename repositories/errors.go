package repositories

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// uniqueViolation is the SQLSTATE Postgres raises when an insert or update
// collides with a unique index.
const uniqueViolation = "23505"

// IsUniqueViolation reports whether err is Postgres rejecting a duplicate.
// Callers use it to tell an expected collision (a taken email, a regenerated
// invitation code, a lost insert race) from a real failure, without matching
// on driver message text that changes between drivers and versions.
func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError

	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
