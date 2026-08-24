package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func UserRouter(r *gin.Engine) {

	userRepo := repositories.NewUserRepository(repositories.GetDB())
	financeRepo := repositories.NewFinanceRepository(repositories.GetDB())
	handler := controllers.NewUserHandler(userRepo, financeRepo)

	auth := r.Group("/auth")
	auth.POST("/login", handler.Login)
	auth.POST("/register", handler.Register)

	users := r.Group("/users")
	users.Use(middlewares.AuthMiddleware())
	{
		users.PATCH("/me", handler.UpdateProfile)
		users.PATCH("/me/password", handler.UpdatePassword)
	}
}
