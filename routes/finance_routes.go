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

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())

	finances := r.Group("/finances")
	finances.Use(middlewares.AuthMiddleware(authRepo), middlewares.FinanceAccess(accessRepo))
	{
		finances.GET("/summary", handler.GetDashboardSummary)
		finances.GET("/breakdown", handler.GetDashboardData)
	}
}
