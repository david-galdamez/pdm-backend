package services

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetFinanceId(c *gin.Context) (uint, error) {
	financeIdParam := c.Query("finance_id")

	if financeIdParam != "" {
		id, err := strconv.ParseUint(financeIdParam, 10, 64)
		if err != nil {
			return 0, err
		}

		return uint(id), nil
	}

	return 0, nil
}
