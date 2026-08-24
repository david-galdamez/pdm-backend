package routes

import (
	"pdm-backend/controllers"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func InvitationRouter(r *gin.Engine) {

	invitationRepo := repositories.NewInvitationRepository(repositories.GetDB())
	handler := controllers.NewInvitationHandler(invitationRepo)

	invitations := r.Group("/invitations")
	invitations.Use(middlewares.AuthMiddleware())
	{
		invitations.POST("/:id", handler.CreateInvite)
	}
}
