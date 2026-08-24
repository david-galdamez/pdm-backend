package main

import (
	"log"
	"os"
	"pdm-backend/routes"
	"pdm-backend/websockets"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {

	if os.Getenv("ENV") != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Println("Could not load .env (this is expected in production)")
		}
	}

	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Authorization"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
		AllowOriginFunc: func(origin string) bool {
			return origin == "" || origin == "null"
		},
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

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	r.Run(":" + port)
}
