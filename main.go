package main

import (
	"pdm-backend/internal/config"
	"pdm-backend/routes"
	"pdm-backend/websockets"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Get()

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:  cfg.ALLOWED_ORIGINS,
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders: []string{"Content-Length", "Authorization"},
		MaxAge:        12 * time.Hour,
	}))
	go websockets.HandleBroadCast()

	routes.UserRouter(r)
	routes.FinanceRouter(r)
	routes.CategoryRouter(r)
	routes.TransactionRouter(r)
	routes.SubcategoryRouter(r)
	routes.IncomeSourceRouter(r)
	routes.SavingRouter(r)
	routes.InvitationRouter(r)
	routes.SharedFinanceRouter(r)
	websockets.WebSocketRouter(r)

	r.Run(":" + cfg.PORT)
}
