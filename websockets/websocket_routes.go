package websockets

import (
	"pdm-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func WebSocketRouter(r *gin.Engine, handler *SharedFinanceWS) {

	webSocket := r.Group("/ws")
	webSocket.Use(middlewares.AuthMiddleware())
	{

		webSocket.GET("/finances/:id", handler.HandleConnection)
	}
}
