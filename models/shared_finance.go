package models

import (
	"time"

	"gorm.io/gorm"
)

type SharedFinance struct {
	gorm.Model
	// One membership row per user per finance: JoinUser reactivates the
	// existing row rather than inserting a second one, and the unique index is
	// what keeps two concurrent joins from both inserting.
	FinanceID uint              `json:"finance_id" gorm:"not null;uniqueIndex:idx_shared_finances_member,priority:1,where:deleted_at IS NULL"`
	Finance   Finance           `gorm:"foreignKey:FinanceID"`
	UserID    uint              `json:"user_id" gorm:"index;not null;uniqueIndex:idx_shared_finances_member,priority:2"`
	User      User              `gorm:"foreignKey:UserID"`
	RoleID    uint              `json:"role_id" gorm:"not null"`
	Role      SharedFinanceRole `gorm:"foreignKey:RoleID" json:"role"`
	Active    bool              `gorm:"not null" json:"active"`
	JoinedAt  time.Time         `json:"joined_at" gorm:"not null"`
}
