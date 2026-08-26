package services

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func ParseMonthAndYear(c *gin.Context) (monthStart, monthEnd time.Time, httpCode int, jsonResponse gin.H, ok bool) {
	monthParam := c.Query("month")
	yearParam := c.Query("year")

	month, err := strconv.Atoi(monthParam)
	if err != nil || month < 1 || month > 12 {
		return time.Time{}, time.Time{}, http.StatusBadRequest, gin.H{"success": false, "message": "Invalid month"}, false
	}

	year, err := strconv.Atoi(yearParam)
	if err != nil || year < 1900 {
		return time.Time{}, time.Time{}, http.StatusBadRequest, gin.H{"success": false, "message": "Invalid year"}, false
	}

	monthStart = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd = monthStart.AddDate(0, 1, 0)

	return monthStart, monthEnd, 0, nil, true
}
