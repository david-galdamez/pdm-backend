package models

import (
	"time"

	"gorm.io/gorm"
)

type Invitation struct {
	gorm.Model
	FinanceID uint      `json:"finance_id" gorm:"index;not null"`
	Code      string    `gorm:"size:16;not null;uniqueIndex" json:"code"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	Finance   Finance   `gorm:"foreignKey:FinanceID" json:"finance"`
}
