package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name         string `json:"name" gorm:"not null"`
	Email        string `json:"email" gorm:"uniqueIndex"`
	PasswordHash string `json:"-" gorm:"size:255;not null"`
	// TokenVersion is embedded in every JWT issued to this user. Bumping it
	// (on password change) makes every outstanding token fail validation
	// immediately, without needing a revocation list.
	TokenVersion   uint            `json:"-" gorm:"not null;default:0"`
	SharedFinances []SharedFinance `json:"shared_finances" gorm:"foreignKey:UserID"`
}
