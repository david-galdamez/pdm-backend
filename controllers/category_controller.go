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

type CategoryHandler struct {
	CategoryRepo *repositories.CategoryRepository
}

func NewCategoryHandler(categoryRepo *repositories.CategoryRepository) *CategoryHandler {
	return &CategoryHandler{CategoryRepo: categoryRepo}
}

func (h *CategoryHandler) GetCategories(c *gin.Context) {

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

	categories, err := h.CategoryRepo.GetCategories(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *CategoryHandler) GetCategoriesData(c *gin.Context) {

	var financeId uint

	categoryId, httpCode, jsonResponse := services.ParseUint(c)
	if categoryId == nil {
		c.JSON(httpCode, jsonResponse)
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

	breakdown, err := h.CategoryRepo.GetCategoriesData(financeId, categoryId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the category data"})
		return
	}

	c.JSON(http.StatusOK, breakdown)
}

type CategoryRequest struct {
	Name string `json:"name"`
}

func (h *CategoryHandler) CreateCategory(c *gin.Context) {

	var financeId uint
	var categoryRequest CategoryRequest
	var category models.ExpenseCategory

	if err := c.ShouldBindJSON(&categoryRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	name := strings.TrimSpace(strings.ToLower(categoryRequest.Name))
	if name == strings.ToLower(models.SavingsCategoryName) {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "You cannot create another category named Savings"})
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

	category.Name = categoryRequest.Name
	category.FinanceID = financeId
	category.UserID = userClaims.UserID

	if err := h.CategoryRepo.CreateCategory(&category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while creating the category"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "The category was created successfully"})
}

func (h *CategoryHandler) UpdateCategory(c *gin.Context) {

	var updateRequest CategoryRequest

	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	categoryId, httpCode, jsonResponse := services.ParseUint(c)
	if categoryId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	category, err := h.CategoryRepo.GetCategoryById(categoryId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "The category was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the category"})
		return
	}

	category.Name = updateRequest.Name

	if err := h.CategoryRepo.UpdateCategory(category); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Error updating the category"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "The category was updated successfully"})
}

func (h *CategoryHandler) GetCategoriesList(c *gin.Context) {

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

	categories, err := h.CategoryRepo.GetCategoriesList(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories, "finance_id": financeId})
}
