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

	savings := r.Group("/savings")
	savings.Use(middlewares.AuthMiddleware())
	{
		savings.GET("", handler.GetSavingsData)
		savings.POST("/goals", handler.CreateSavingGoal)
	}
}
