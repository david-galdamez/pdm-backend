package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"pdm-backend/internal/config"
	"pdm-backend/middlewares"
	"pdm-backend/repositories"
	"pdm-backend/routes"
	"pdm-backend/websockets"
	"sync"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

var wg sync.WaitGroup

func main() {
	cfg := config.Get()

	if cfg.ENV == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()
	api := r.Group("/api")

	api.Use(cors.New(cors.Config{
		AllowOrigins:  cfg.ALLOWED_ORIGINS,
		AllowMethods:  []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowHeaders:  []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders: []string{"Content-Length", "Authorization"},
		MaxAge:        12 * time.Hour,
	}))

	go middlewares.CleanupExpiredClients()

	api.GET("/health", func(c *gin.Context) {
		sqlDB, err := repositories.GetDB().DB()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database connection error"})
			return
		}

		if err := sqlDB.Ping(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database ping error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Server is healthy"})
	})

	doneWS := make(chan struct{})

	sharedFinanceRepo := repositories.NewSharedFinanceRepository(repositories.GetDB())
	handler := websockets.NewSharedFinanceWS(sharedFinanceRepo, &wg, doneWS)
	go handler.HandleBroadCast()

	routes.UserRouter(api)
	routes.FinanceRouter(api)
	routes.CategoryRouter(api)
	routes.TransactionRouter(api)
	routes.SubcategoryRouter(api)
	routes.IncomeSourceRouter(api)
	routes.SavingRouter(api)
	routes.InvitationRouter(api)
	routes.SharedFinanceRouter(api)
	websockets.WebSocketRouter(api, handler)

	s := &http.Server{
		Addr:         ":" + cfg.PORT,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("\nListen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("\nShutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	close(doneWS)
	if err := s.Shutdown(ctx); err != nil {
		log.Println("HTTP Server shutdown:", err)
	}

	// Channel that tracks when the websocket connections are closed
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All websocket connections closed")
	case <-ctx.Done():
		log.Println("Shutdown timeout reached, forcing websocket connections to close")
	}
}
