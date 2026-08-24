package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func SharedFinanceRouter(r *gin.Engine) {

	sharedFinanceRepo := repositories.NewSharedFinanceRepository(repositories.GetDB())
	handler := controllers.NewSharedFinanceHandler(sharedFinanceRepo)

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())
	financeRepo := repositories.NewFinanceRepository(repositories.GetDB())

	sharedFinances := r.Group("/shared-finances")
	sharedFinances.Use(middlewares.AuthMiddleware(authRepo))
	{
		sharedFinances.GET("", handler.GetSharedFinances)
		sharedFinances.GET("/:id", middlewares.FinanceAccessFromParam(accessRepo, "id"), handler.GetSharedFinanceDetails)
		sharedFinances.POST("", handler.CreateSharedFinance)
		sharedFinances.POST("/join", handler.JoinUser)
		sharedFinances.DELETE("/:id/leave", middlewares.FinanceAccessFromParam(accessRepo, "id"), handler.LeaveSharedFinance)
		sharedFinances.DELETE("/members/:id",
			middlewares.FinanceAccess(accessRepo),
			middlewares.RequireFinanceAdmin(financeRepo),
			handler.RemoveUserFromFinance)
	}
}
