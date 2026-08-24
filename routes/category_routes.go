package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func CategoryRouter(r *gin.Engine) {

	categoryRepo := repositories.NewCategoryRepository(repositories.GetDB())
	handler := controllers.NewCategoryHandler(categoryRepo)

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())

	categories := r.Group("/categories")
	categories.Use(middlewares.AuthMiddleware(authRepo), middlewares.FinanceAccess(accessRepo))
	{
		categories.GET("/options", handler.GetCategories)
		categories.GET("", handler.GetCategoriesList)
		categories.GET("/:id/breakdown", handler.GetCategoriesData)
		categories.POST("", handler.CreateCategory)
		categories.PATCH("/:id", handler.UpdateCategory)
	}
}
