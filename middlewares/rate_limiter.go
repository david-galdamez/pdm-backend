package middlewares

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type client struct {
	limiter *rate.Limiter
	expiry  time.Time
}

var (
	mu      sync.Mutex
	clients = make(map[string]*client)
)

func RateLimiter() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		if _, exist := clients[ip]; !exist {
			clients[ip] = &client{
				limiter: rate.NewLimiter(1, 5), // 1 request per second with a burst of 5
				expiry:  time.Now().Add(1 * time.Minute),
			}
		}
		cl := clients[ip]
		cl.expiry = time.Now().Add(1 * time.Minute) // Extend expiry on each request
		mu.Unlock()

		if !cl.limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"message": "Limit exceeded. Please try again later.",
			})
			return
		}

		c.Next()
	}
}

func CleanupExpiredClients() {
	for {
		time.Sleep(1 * time.Minute)
		mu.Lock()
		for ip, cl := range clients {
			if time.Now().After(cl.expiry) {
				delete(clients, ip)
			}
		}
		mu.Unlock()

	}
}
