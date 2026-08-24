package controllers

import (
	"net/http"
	"pdm-backend/repositories"
	"pdm-backend/services"

	"github.com/gin-gonic/gin"
)

type InvitationHandler struct {
	InvitationRepo *repositories.InvitationRepository
}

func NewInvitationHandler(invitationRepo *repositories.InvitationRepository) *InvitationHandler {
	return &InvitationHandler{InvitationRepo: invitationRepo}
}

func (h *InvitationHandler) CreateInvite(c *gin.Context) {

	financeId, httpCode, jsonResponse := services.ParseUint(c)
	if financeId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	invitation, err := h.InvitationRepo.CreateInvite(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while creating the invitation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "invitation_code": invitation.Code})
}
