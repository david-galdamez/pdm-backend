package repositories

import (
	"errors"
	"pdm-backend/models"
	"pdm-backend/services"
	"strings"
	"time"

	"gorm.io/gorm"
)

type InvitationRepository struct {
	DB *gorm.DB
}

func NewInvitationRepository(db *gorm.DB) *InvitationRepository {
	return &InvitationRepository{DB: db}
}

type Invitation struct {
	Code string `json:"invitation_code"`
}

func (r *InvitationRepository) CreateInvite(financeId *uint) (*Invitation, error) {

	var response Invitation
	maxAttempts := 5

	for range maxAttempts {

		code, err := services.GenerateInvitationCode(10)
		if err != nil {
			return nil, err
		}

		invitation := models.Invitation{
			FinanceID: *financeId,
			Code:      code,
			ExpiresAt: time.Now().Add(time.Minute * 15),
		}

		err = r.DB.Create(&invitation).Error
		if err == nil {
			response.Code = invitation.Code
			return &response, nil
		}

		// A collision on the generated code is worth another attempt; anything
		// else is a real failure.
		if !strings.Contains(err.Error(), "UNIQUE constraint failed") && !strings.Contains(err.Error(), "duplicate key") {
			return nil, err
		}
	}

	return nil, errors.New("could not generate a unique invitation code")
}
