package controllers

import (
	"errors"
	"net/http"
	"pdm-backend/models"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"pdm-backend/websockets"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TransactionHandler struct {
	TransactionRepo *repositories.TransactionRepository
}

func NewTransactionHandler(transactionRepo *repositories.TransactionRepository) *TransactionHandler {
	return &TransactionHandler{TransactionRepo: transactionRepo}
}

func (h *TransactionHandler) GetTransactions(c *gin.Context) {

	financeId := services.FinanceId(c)

	monthStart, monthEnd, httpCode, jsonResponse, ok := services.ParseMonthAndYear(c)
	if !ok {
		c.JSON(httpCode, jsonResponse)
		return
	}

	month := int(monthStart.Month())
	year := monthStart.Year()

	transactions, err := h.TransactionRepo.GetTransactions(monthStart, monthEnd, financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the transactions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"finance_id":   financeId,
		"month":        month,
		"year":         year,
		"transactions": transactions,
	})
}

func (h *TransactionHandler) GetTransactionOptions(c *gin.Context) {

	financeId := services.FinanceId(c)

	options, err := h.TransactionRepo.GetOptions(financeId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the transaction options"})
		return
	}

	c.JSON(http.StatusOK, options)
}

func (h *TransactionHandler) GetTransactionById(c *gin.Context) {

	transactionId, httpCode, jsonResponse := services.ParseUint(c)
	if transactionId == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeId := services.FinanceId(c)

	transaction, err := h.TransactionRepo.GetTransactionById(transactionId, financeId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "The transaction was not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while fetching the transaction"})
		return
	}

	c.JSON(http.StatusOK, transaction)
}

type TransactionRequest struct {
	EntryTypeID uint       `json:"entry_type_id" binding:"required"`
	MovementID  uint       `json:"movement_id" binding:"required"`
	Amount      float64    `json:"amount" binding:"required,gt=0"`
	Description string     `json:"description"`
	OccurredAt  *time.Time `json:"occurred_at" binding:"required"`
}

func (h *TransactionHandler) CreateTransaction(c *gin.Context) {

	var transactionRequest TransactionRequest
	var transaction models.Transaction

	now := time.Now()
	minimumDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	if err := c.ShouldBindJSON(&transactionRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "The request format is invalid"})
		return
	}

	userClaims, httpCode, jsonResponse := services.GetClaims(c)
	if userClaims == nil {
		c.JSON(httpCode, jsonResponse)
		return
	}

	financeId := services.FinanceId(c)
	savingsId := userClaims.SavingsID

	// Any finance other than the caller's own personal one is shared, and its
	// savings subcategory is not the one carried in the token.
	isSharedFinance := financeId != userClaims.FinanceID

	if isSharedFinance {
		subcategoryId, err := h.TransactionRepo.GetSavingSubcategory(financeId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while resolving the savings subcategory"})
			return
		}

		savingsId = subcategoryId
	}

	transaction.FinanceID = financeId
	transaction.UserID = userClaims.UserID
	transaction.EntryTypeID = transactionRequest.EntryTypeID
	transaction.Amount = transactionRequest.Amount
	transaction.Description = &transactionRequest.Description

	date := transactionRequest.OccurredAt

	if transactionRequest.OccurredAt == nil {
		transactionRequest.OccurredAt = &now
	} else {
		if date.After(now) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "The transaction date cannot be in the future",
			})
			return
		}

		if date.Before(minimumDate) {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "The transaction date is too far in the past",
			})
			return
		}

		dateYear, dateMonth, _ := date.Date()
		nowYear, nowMonth, _ := now.Date()

		if dateYear != nowYear || dateMonth != nowMonth {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "You can only record transactions for the current month",
			})
			return
		}
	}

	transaction.OccurredAt = *transactionRequest.OccurredAt

	switch transactionRequest.EntryTypeID {
	case models.EntryTypeIncome:
		// The movement id comes from the client, so it has to be confirmed
		// against this finance before it is written onto the transaction.
		belongs, err := h.TransactionRepo.IncomeSourceBelongsToFinance(transactionRequest.MovementID, financeId)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while resolving the income source"})
			return
		}

		if !belongs {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "That income source was not found"})
			return
		}

		transaction.IncomeSourceID = &transactionRequest.MovementID

	case models.EntryTypeExpense:
		transaction.ExpenseSubcategoryID = &transactionRequest.MovementID

		identifiers, err := h.TransactionRepo.GetIds(*transaction.ExpenseSubcategoryID, financeId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "That subcategory was not found"})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while resolving the category identifiers"})
			return
		}

		transaction.ExpenseCategoryID = &identifiers.CategoryID
		transaction.BudgetTypeID = &identifiers.BudgetTypeID

	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid transaction type"})
		return
	}

	isSaving := transaction.ExpenseSubcategoryID != nil && *transaction.ExpenseSubcategoryID == savingsId

	if err := h.TransactionRepo.CreateTransactionWithSaving(&transaction, isSaving); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while creating the transaction"})
		return
	}

	if isSharedFinance {
		webSocketEvent := h.TransactionRepo.BuildWebSocketEvent(financeId, transaction.ExpenseSubcategoryID, savingsId)

		websockets.BroadcastMessages <- *webSocketEvent
	}

	c.JSON(http.StatusCreated, gin.H{"success": true, "message": "The transaction was created successfully"})
}
