package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"pdm-backend/internal/config"
	"pdm-backend/repositories"
	"pdm-backend/routes"
	"pdm-backend/websockets"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Get()

	if cfg.ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:  cfg.ALLOWED_ORIGINS,
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders: []string{"Content-Length", "Authorization"},
		MaxAge:        12 * time.Hour,
	}))

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	sharedFinanceRepo := repositories.NewSharedFinanceRepository(repositories.GetDB())
	handler := websockets.NewSharedFinanceWS(sharedFinanceRepo)
	go handler.HandleBroadCast()

	routes.UserRouter(r)
	routes.FinanceRouter(r)
	routes.CategoryRouter(r)
	routes.TransactionRouter(r)
	routes.SubcategoryRouter(r)
	routes.IncomeSourceRouter(r)
	routes.SavingRouter(r)
	routes.InvitationRouter(r)
	routes.SharedFinanceRouter(r)
	websockets.WebSocketRouter(r, handler)

	s := &http.Server{
		Addr:         ":" + cfg.PORT,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		log.Println("Server shutdown:", err)
	}
	log.Println("Server exiting")
}
