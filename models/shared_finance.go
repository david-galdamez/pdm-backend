package models

import (
	"time"

	"gorm.io/gorm"
)

type SharedFinance struct {
	gorm.Model
	FinanceID uint              `json:"finance_id" gorm:"index;not null"`
	Finance   Finance           `gorm:"foreignKey:FinanceID"`
	UserID    uint              `json:"user_id" gorm:"index;not null"`
	User      User              `gorm:"foreignKey:UserID"`
	RoleID    uint              `json:"role_id" gorm:"not null"`
	Role      SharedFinanceRole `gorm:"foreignKey:RoleID" json:"role"`
	Active    bool              `gorm:"not null" json:"active"`
	JoinedAt  time.Time         `json:"joined_at" gorm:"not null"`
}
