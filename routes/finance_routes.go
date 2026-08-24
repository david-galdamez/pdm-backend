package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func FinanceRouter(r *gin.Engine) {

	financeRepo := repositories.NewFinanceRepository(repositories.GetDB())
	handler := controllers.NewFinanceHandler(financeRepo)

	finances := r.Group("/finances")
	finances.Use(middlewares.AuthMiddleware())
	{
		finances.GET("/summary", handler.GetDashboardSummary)
		finances.GET("/breakdown", handler.GetDashboardData)
	}
}
