package models

import (
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Name           string          `json:"name" gorm:"not null"`
	Email          string          `json:"email" gorm:"uniqueIndex"`
	PasswordHash   string          `json:"-" gorm:"size:255;not null"`
	SharedFinances []SharedFinance `json:"shared_finances" gorm:"foreignKey:UserID"`
}
