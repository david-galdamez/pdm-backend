package controllers

import (
	"net/http"
	"pdm-backend/repositories"
	"pdm-backend/services"

	"github.com/gin-gonic/gin"
)

type FinanceHandler struct {
	FinanceRepo *repositories.FinanceRepository
}

func NewFinanceHandler(financeRepo *repositories.FinanceRepository) *FinanceHandler {
	return &FinanceHandler{FinanceRepo: financeRepo}
}

func (h *FinanceHandler) GetDashboardSummary(c *gin.Context) {

	financeId := services.FinanceId(c)

	monthStart, monthEnd, httpCode, jsonResponse, ok := services.ParseMonthAndYear(c)
	if !ok {
		c.JSON(httpCode, jsonResponse)
		return
	}

	summary, err := h.FinanceRepo.GetDashboardSummary(financeId, monthStart, monthEnd)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while building the dashboard summary"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"finance_id": financeId,
		"summary":    summary,
	})
}

func (h *FinanceHandler) GetDashboardData(c *gin.Context) {

	financeId := services.FinanceId(c)

	monthStart, monthEnd, httpCode, jsonResponse, ok := services.ParseMonthAndYear(c)
	if !ok {
		c.JSON(httpCode, jsonResponse)
		return
	}

	result, err := h.FinanceRepo.GetDataSummary(monthStart, monthEnd, financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while building the dashboard data"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"categories": result,
	})
}
