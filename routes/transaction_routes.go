package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func TransactionRouter(r *gin.RouterGroup) {
	transactionRepo := repositories.NewTransactionRepository(repositories.GetDB())
	handler := controllers.NewTransactionHandler(transactionRepo)

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())

	transactions := r.Group("/transactions")
	transactions.Use(middlewares.AuthMiddleware(authRepo), middlewares.FinanceAccess(accessRepo))
	{
		transactions.GET("", handler.GetTransactions)
		transactions.GET("/options", handler.GetTransactionOptions)
		transactions.GET("/:id", handler.GetTransactionById)
		transactions.POST("", handler.CreateTransaction)
	}
}
