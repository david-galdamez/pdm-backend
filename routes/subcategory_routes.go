package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func SubcategoryRouter(r *gin.Engine) {

	subcategoryRepo := repositories.NewSubcategoryRepository(repositories.GetDB())
	handler := controllers.NewSubcategoryHandler(subcategoryRepo)

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())

	subcategories := r.Group("/subcategories")
	subcategories.Use(middlewares.AuthMiddleware(authRepo), middlewares.FinanceAccess(accessRepo))
	{
		subcategories.GET("", handler.GetSubcategoriesList)
		subcategories.GET("/budget-types", handler.GetBudgetTypes)
		subcategories.GET("/:id", handler.GetSubcategoryById)
		subcategories.POST("", handler.CreateSubcategory)
		subcategories.PUT("/:id", handler.UpdateSubcategory)
	}
}
