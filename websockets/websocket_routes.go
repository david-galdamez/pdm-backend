package websockets

import (
	"pdm-backend/middlewares"
	"pdm-backend/repositories"

	"github.com/gin-gonic/gin"
)

func WebSocketRouter(r *gin.Engine, handler *SharedFinanceWS) {

	authRepo := repositories.NewUserRepository(repositories.GetDB())

	webSocket := r.Group("/ws")
	webSocket.Use(middlewares.AuthMiddleware(authRepo))
	{

		webSocket.GET("/finances/:id", handler.HandleConnection)
	}
}
