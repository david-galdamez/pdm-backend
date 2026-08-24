package controllers

import (
	"net/http"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type SavingHandler struct {
	SavingRepo *repositories.SavingRepository
}

func NewSavingHandler(savingRepo *repositories.SavingRepository) *SavingHandler {
	return &SavingHandler{SavingRepo: savingRepo}
}

func (h *SavingHandler) GetSavingsData(c *gin.Context) {

	var financeId uint
	yearParam := c.Query("year")

	year, err := strconv.Atoi(yearParam)
	if err != nil || year < 2025 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The year cannot be earlier than the current one"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	id, err := services.GetFinanceId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The query format is invalid"})
		return
	}

	financeId = userClaims.FinanceID

	if id != 0 {
		financeId = id
	}

	savingsData, err := h.SavingRepo.GetSavingsData(financeId, year)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the savings data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "finance_id": financeId, "data": savingsData})
}

type SavingGoalRequest struct {
	Amount float64 `json:"amount"`
	Month  int     `json:"month"`
	Year   int     `json:"year"`
}

func (h *SavingHandler) CreateSavingGoal(c *gin.Context) {

	var financeId uint
	var goalRequest SavingGoalRequest

	if err := c.ShouldBindJSON(&goalRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	now := time.Now()
	currentMonth := int(now.Month())

	if goalRequest.Month < currentMonth {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The month cannot be earlier than the current one"})
		return
	}

	if goalRequest.Month < 1 || goalRequest.Month > 12 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Enter a valid month"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	id, err := services.GetFinanceId(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The query format is invalid"})
		return
	}

	financeId = userClaims.FinanceID

	if id != 0 {
		financeId = id
	}

	if err := h.SavingRepo.CreateOrUpdateSavingGoal(financeId, goalRequest.Amount, goalRequest.Month, goalRequest.Year); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while saving the monthly goal"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "The goal was created or updated successfully"})
}
