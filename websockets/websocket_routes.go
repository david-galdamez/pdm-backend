package websockets

import (
	"pdm-backend/middlewares"

	"github.com/gin-gonic/gin"
)

func WebSocketRouter(r *gin.Engine) {

	webSocket := r.Group("/ws")
	webSocket.Use(middlewares.AuthMiddleware())
	{

		webSocket.GET("/finances/:id", func(c *gin.Context) {
			HandleConnection(c)
		})
	}
}
