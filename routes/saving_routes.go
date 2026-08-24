package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func SavingRouter(r *gin.Engine) {

	savingRepo := repositories.NewSavingRepository(repositories.GetDB())
	handler := controllers.NewSavingHandler(savingRepo)

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())

	savings := r.Group("/savings")
	savings.Use(middlewares.AuthMiddleware(authRepo), middlewares.FinanceAccess(accessRepo))
	{
		savings.GET("", handler.GetSavingsData)
		savings.POST("/goals", handler.CreateSavingGoal)
	}
}
