package services

import (
	"github.com/gin-gonic/gin"
)

// FinanceIdKey is where middlewares.FinanceAccess stores the finance a request
// was authorized against.
const FinanceIdKey = "financeId"

// FinanceId returns that finance. It is only meaningful on routes that run
// middlewares.FinanceAccess, which aborts the request when the caller has no
// access, so handlers can use the result without checking it again.
func FinanceId(c *gin.Context) uint {
	value, exists := c.Get(FinanceIdKey)
	if !exists {
		return 0
	}

	financeId, _ := value.(uint)

	return financeId
}
