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

	sharedFinances := r.Group("/shared-finances")
	sharedFinances.Use(middlewares.AuthMiddleware())
	{
		sharedFinances.GET("", handler.GetSharedFinances)
		sharedFinances.GET("/:id", handler.GetSharedFinanceDetails)
		sharedFinances.POST("", handler.CreateSharedFinance)
		sharedFinances.POST("/join", handler.JoinUser)
		sharedFinances.DELETE("/:id/leave", handler.LeaveSharedFinance)
		sharedFinances.DELETE("/members/:id", handler.RemoveUserFromFinance)
	}
}
