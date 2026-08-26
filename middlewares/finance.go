package middlewares

import (
	"net/http"
	"pdm-backend/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

// FinanceAccessChecker reports whether a user may act on a finance. It is an
// interface so the middleware does not depend on the repositories package.
type FinanceAccessChecker interface {
	CanAccessFinance(userId, financeId uint) (bool, error)
}

// FinanceAccess resolves the finance a request targets — the finance_id query
// parameter when present, the caller's personal finance otherwise — and aborts
// unless the caller may act on it. Handlers read the result with
// services.FinanceId instead of trusting the query parameter themselves.
func FinanceAccess(checker FinanceAccessChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userClaims, httpCode, jsonResponse := services.GetClaims(c)
		if userClaims == nil {
			c.AbortWithStatusJSON(httpCode, jsonResponse)
			return
		}

		financeId := userClaims.FinanceID

		if financeIdParam := c.Query("finance_id"); financeIdParam != "" {
			id, err := strconv.ParseUint(financeIdParam, 10, 64)
			if err != nil || id == 0 {
				c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "message": "The finance id is not a valid number"})
				return
			}

			financeId = uint(id)
		}

		authorizeFinance(c, checker, userClaims.UserID, financeId)
	}
}

// FinanceAccessFromParam is the same check for routes such as
// /shared-finances/:id that name the finance in the path rather than the query
// string.
func FinanceAccessFromParam(checker FinanceAccessChecker, param string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userClaims, httpCode, jsonResponse := services.GetClaims(c)
		if userClaims == nil {
			c.AbortWithStatusJSON(httpCode, jsonResponse)
			return
		}

		id, err := strconv.ParseUint(c.Param(param), 10, 64)
		if err != nil || id == 0 {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"success": false, "message": "The finance id is not a valid number"})
			return
		}

		authorizeFinance(c, checker, userClaims.UserID, uint(id))
	}
}

func authorizeFinance(c *gin.Context, checker FinanceAccessChecker, userId, financeId uint) {

	allowed, err := checker.CanAccessFinance(userId, financeId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while checking your access to the finance"})
		return
	}

	if !allowed {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "message": "You don't have access to this finance"})
		return
	}

	c.Set(services.FinanceIdKey, financeId)

	c.Next()
}

// FinanceAdminChecker reports whether a user administers a finance. It is an
// interface so the middleware does not depend on the repositories package.
type FinanceAdminChecker interface {
	IsFinanceAdmin(financeId, userId uint) (bool, error)
}

// RequireFinanceAdmin aborts unless the caller administers the finance a
// prior FinanceAccess or FinanceAccessFromParam resolved, so it must run after
// one of those in the chain: it reads services.FinanceId(c) rather than
// re-deriving the finance itself.
func RequireFinanceAdmin(checker FinanceAdminChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userClaims, httpCode, jsonResponse := services.GetClaims(c)
		if userClaims == nil {
			c.AbortWithStatusJSON(httpCode, jsonResponse)
			return
		}

		financeId := services.FinanceId(c)

		isAdmin, err := checker.IsFinanceAdmin(financeId, userClaims.UserID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"success": false, "message": "An error occurred while checking your access to the finance"})
			return
		}

		if !isAdmin {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"success": false, "message": "Finance not found or you're not its admin"})
			return
		}

		c.Next()
	}
}
