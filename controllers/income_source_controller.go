package controllers

import (
	"errors"
	"net/http"
	"pdm-backend/models"
	"pdm-backend/repositories"
	"pdm-backend/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type IncomeSourceHandler struct {
	IncomeSourceRepo *repositories.IncomeSourceRepository
}

func NewIncomeSourceHandler(incomeSourceRepo *repositories.IncomeSourceRepository) *IncomeSourceHandler {
	return &IncomeSourceHandler{IncomeSourceRepo: incomeSourceRepo}
}

func (h *IncomeSourceHandler) GetIncomeSourcesList(c *gin.Context) {

	financeId := services.FinanceId(c)

	incomeSources, err := h.IncomeSourceRepo.GetIncomeSourcesList(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the income sources"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "income_sources": incomeSources})
}

func (h *IncomeSourceHandler) GetIncomeSourceById(c *gin.Context) {

	incomeSourceId, httpCode, jsonResponse := services.ParseUint(c)
	if incomeSourceId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeId := services.FinanceId(c)

	incomeSource, err := h.IncomeSourceRepo.GetIncomeSource(incomeSourceId, financeId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "That income source does not exist"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the income source"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "income_source": incomeSource})
}

type IncomeSourceRequest struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
}

func (h *IncomeSourceHandler) CreateIncomeSource(c *gin.Context) {

	var incomeSourceRequest IncomeSourceRequest
	var incomeSource models.IncomeSource

	if err := c.ShouldBindJSON(&incomeSourceRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeId := services.FinanceId(c)

	incomeSource.FinanceID = financeId
	incomeSource.UserID = userClaims.UserID
	incomeSource.Name = incomeSourceRequest.Name
	incomeSource.Description = incomeSourceRequest.Description
	incomeSource.Amount = incomeSourceRequest.Amount

	if err := h.IncomeSourceRepo.CreateIncomeSource(&incomeSource); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while creating the income source"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "Income source created successfully"})
}

func (h *IncomeSourceHandler) UpdateIncomeSource(c *gin.Context) {

	var incomeSourceRequest IncomeSourceRequest

	incomeSourceId, httpCode, jsonResponse := services.ParseUint(c)
	if incomeSourceId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	if err := c.ShouldBindJSON(&incomeSourceRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	financeId := services.FinanceId(c)

	incomeSource, err := h.IncomeSourceRepo.GetIncomeSourceById(incomeSourceId, financeId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "That income source does not exist"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the income source"})
		return
	}

	incomeSource.Name = incomeSourceRequest.Name
	incomeSource.Description = incomeSourceRequest.Description
	incomeSource.Amount = incomeSourceRequest.Amount

	if err := h.IncomeSourceRepo.UpdateIncomeSource(incomeSource); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while updating the income source"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "The income source was updated successfully"})
}
