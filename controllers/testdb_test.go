package controllers

import (
	"database/sql"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// failingDB hands back a gorm handle whose every query fails. It reuses
// lazyConnector but leaves DryRun off, so a handler that reaches the database
// really tries and really fails — which is what the cases about "the write did
// not happen" need to be sure of.
func failingDB(t *testing.T) *gorm.DB {
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
