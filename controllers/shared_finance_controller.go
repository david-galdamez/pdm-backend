package controllers

import (
	"errors"
	"net/http"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"pdm-backend/websockets"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SharedFinanceHandler struct {
	SharedFinanceRepo *repositories.SharedFinanceRepository
}

func NewSharedFinanceHandler(sharedFinanceRepo *repositories.SharedFinanceRepository) *SharedFinanceHandler {
	return &SharedFinanceHandler{SharedFinanceRepo: sharedFinanceRepo}
}

type CreateSharedFinanceRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

func (h *SharedFinanceHandler) CreateSharedFinance(c *gin.Context) {

	var createRequest CreateSharedFinanceRequest

	if err := c.ShouldBindJSON(&createRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	err := h.SharedFinanceRepo.CreateSharedFinance(userClaims.UserID, createRequest.Title, createRequest.Description)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while creating the finance"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "The finance was created successfully"})
}

type JoinRequest struct {
	Code string `json:"code"`
}

func (h *SharedFinanceHandler) JoinUser(c *gin.Context) {
	var joinRequest JoinRequest

	if err := c.ShouldBindJSON(&joinRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	err := h.SharedFinanceRepo.JoinUser(userClaims.UserID, joinRequest.Code)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "That invitation code does not exist"})
			return
		}

		if errors.Is(err, repositories.ErrAlreadyMember) || errors.Is(err, repositories.ErrInviteExpired) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while joining the finance"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "You joined the finance successfully"})
}

func (h *SharedFinanceHandler) GetSharedFinances(c *gin.Context) {

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	sharedFinances, err := h.SharedFinanceRepo.GetSharedFinances(userClaims.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the finances"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "finances": sharedFinances})
}

func (h *SharedFinanceHandler) GetSharedFinanceDetails(c *gin.Context) {

	financeId := services.FinanceId(c)

	details, err := h.SharedFinanceRepo.GetSharedFinanceDetails(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the finance details"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "finance_id": financeId, "details": details})
}

func (h *SharedFinanceHandler) RemoveUserFromFinance(c *gin.Context) {

	userId, httpCode, jsonResponse := services.ParseUint(c)
	if userId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeId := services.FinanceId(c)

	if err := h.SharedFinanceRepo.LeaveSharedFinance(*userId, financeId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "That user is not a member of this finance"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while removing the user"})
		return
	}

	websockets.DisconnectUser(financeId, *userId)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "The user was removed successfully"})
}

func (h *SharedFinanceHandler) LeaveSharedFinance(c *gin.Context) {

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeId, httpCode, jsonResponse := services.ParseUint(c)
	if financeId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	if err := h.SharedFinanceRepo.LeaveSharedFinance(userClaims.UserID, *financeId); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "You are not a member of this finance"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while leaving the finance"})
		return
	}

	websockets.DisconnectUser(*financeId, userClaims.UserID)

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "You left the finance successfully"})
}
