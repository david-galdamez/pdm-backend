package middlewares

import (
	"net/http"
	"pdm-backend/services"
	"strings"

	"github.com/gin-gonic/gin"
)

// TokenVersionChecker reports the version currently stamped on a user, so a
// token whose embedded version has fallen behind (the user changed their
// password since it was issued) can be rejected even though it has not
// expired yet.
type TokenVersionChecker interface {
	GetTokenVersion(userId uint) (uint, error)
}

func AuthMiddleware(checker TokenVersionChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Token not provided"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid token format"})
			c.Abort()
			return
		}

		_, claims, err := services.ValidateJWT(tokenString)

		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Invalid or expired token"})
			c.Abort()
			return
		}

		currentVersion, err := checker.GetTokenVersion(claims.UserID)
		if err != nil || currentVersion != claims.TokenVersion {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Your session has expired, please log in again"})
			c.Abort()
			return
		}

		c.Set("claims", claims)

		c.Next()
	}
}
