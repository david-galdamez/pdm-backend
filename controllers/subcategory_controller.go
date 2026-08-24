package controllers

import (
	"errors"
	"net/http"
	"pdm-backend/models"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SubcategoryHandler struct {
	SubcategoryRepo *repositories.SubcategoryRepository
}

func NewSubcategoryHandler(subcategoryRepo *repositories.SubcategoryRepository) *SubcategoryHandler {
	return &SubcategoryHandler{SubcategoryRepo: subcategoryRepo}
}

func (h *SubcategoryHandler) GetSubcategoryById(c *gin.Context) {

	subcategoryId, httpCode, jsonResponse := services.ParseUint(c)
	if subcategoryId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	subcategory, err := h.SubcategoryRepo.GetSubcategory(subcategoryId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "The subcategory was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the subcategory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "subcategory": subcategory})
}

func (h *SubcategoryHandler) GetBudgetTypes(c *gin.Context) {

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	budgetTypes, err := h.SubcategoryRepo.GetBudgetTypes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the budget types"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"options": budgetTypes})
}

func (h *SubcategoryHandler) GetSubcategoriesList(c *gin.Context) {

	var financeId uint

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

	subcategories, err := h.SubcategoryRepo.GetSubcategoriesList(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the subcategories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "subcategories": subcategories})
}

type SubcategoryRequest struct {
	CategoryID   uint    `json:"category_id" binding:"required,gt=0"`
	Name         string  `json:"name" binding:"required"`
	BudgetTypeID uint    `json:"budget_type_id" binding:"required,gt=0"`
	Budget       float64 `json:"budget" binding:"required,gte=0"`
}

func (h *SubcategoryHandler) CreateSubcategory(c *gin.Context) {
	var financeId uint
	var subcategoryRequest SubcategoryRequest
	var subcategory models.ExpenseSubcategory

	if err := c.ShouldBindJSON(&subcategoryRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	name := strings.TrimSpace(strings.ToLower(subcategoryRequest.Name))
	if name == strings.ToLower(models.SavingsCategoryName) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "You cannot create another subcategory named Savings"})
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

	subcategory.FinanceID = financeId
	subcategory.UserID = userClaims.UserID
	subcategory.ExpenseCategoryID = subcategoryRequest.CategoryID
	subcategory.Name = subcategoryRequest.Name
	subcategory.MonthlyBudget = subcategoryRequest.Budget
	subcategory.BudgetTypeID = subcategoryRequest.BudgetTypeID

	if err := h.SubcategoryRepo.CreateSubcategory(&subcategory); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while creating the subcategory"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "The subcategory was created successfully"})
}

func (h *SubcategoryHandler) UpdateSubcategory(c *gin.Context) {

	var updateRequest SubcategoryRequest

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	subcategoryId, httpCode, jsonResponse := services.ParseUint(c)
	if subcategoryId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	subcategory, err := h.SubcategoryRepo.GetSubcategoryById(subcategoryId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "The subcategory was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the subcategory"})
		return
	}

	subcategory.ExpenseCategoryID = updateRequest.CategoryID
	subcategory.Name = updateRequest.Name
	subcategory.MonthlyBudget = updateRequest.Budget
	subcategory.BudgetTypeID = updateRequest.BudgetTypeID

	if err := h.SubcategoryRepo.UpdateSubcategory(subcategory); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error updating the subcategory"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "The subcategory was updated successfully"})
}
