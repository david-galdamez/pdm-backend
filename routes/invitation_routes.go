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

	authRepo := repositories.NewUserRepository(repositories.GetDB())
	accessRepo := repositories.NewFinanceAccessRepository(repositories.GetDB())
	financeRepo := repositories.NewFinanceRepository(repositories.GetDB())

	invitations := r.Group("/invitations")
	invitations.Use(middlewares.AuthMiddleware(authRepo))
	{
		invitations.POST("/:id",
			middlewares.FinanceAccessFromParam(accessRepo, "id"),
			middlewares.RequireFinanceAdmin(financeRepo),
			handler.CreateInvite)
	}
}
