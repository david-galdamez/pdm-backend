package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func IncomeSourceRouter(r *gin.Engine) {

	incomeSourceRepo := repositories.NewIncomeSourceRepository(repositories.GetDB())
	handler := controllers.NewIncomeSourceHandler(incomeSourceRepo)

	incomeSources := r.Group("/income-sources")
	incomeSources.Use(middlewares.AuthMiddleware())
	{
		incomeSources.GET("", handler.GetIncomeSourcesList)
		incomeSources.GET("/:id", handler.GetIncomeSourceById)
		incomeSources.POST("", handler.CreateIncomeSource)
		incomeSources.PUT("/:id", handler.UpdateIncomeSource)
	}
}
