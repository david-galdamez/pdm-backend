package controllers

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"pdm-backend/repositories"
	"pdm-backend/services"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// lazyConnector satisfies database/sql without ever dialing, so the handler
// can run against a dry-run gorm and be judged on the status it returns rather
// than on anything it writes.
type lazyConnector struct{}

func (lazyConnector) Connect(context.Context) (driver.Conn, error) {
	return nil, errors.New("lazyConnector never connects")
}

func (lazyConnector) Driver() driver.Driver { return nil }

func dryRunSavingHandler(t *testing.T) *SavingHandler {
	t.Helper()

	db, err := gorm.Open(
		postgres.New(postgres.Config{Conn: sql.OpenDB(lazyConnector{})}),
		&gorm.Config{
			DryRun:               true,
			DisableAutomaticPing: true,
			// A dry run cannot execute, so the reads log an error on the way
			// out. The cases here judge the handler on its status code.
			Logger: logger.Default.LogMode(logger.Silent),
		},
	)
	if err != nil {
		t.Fatalf("opening dry-run database: %v", err)
	}

	return NewSavingHandler(repositories.NewSavingRepository(db))
}

// postSavingGoal runs CreateSavingGoal against the given body and reports the
// status and message it produced.
func postSavingGoal(t *testing.T, body string) (int, string) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/savings/goal", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	// FinanceAccess would have put this here in a real request.
	c.Set(services.FinanceIdKey, uint(1))

	dryRunSavingHandler(t).CreateSavingGoal(c)

	var response struct {
		Message string `json:"message"`
	}

	_ = json.Unmarshal(recorder.Body.Bytes(), &response)

	return recorder.Code, response.Message
}

func goalBody(amount float64, month, year int) string {
	return fmt.Sprintf(`{"amount":%v,"month":%d,"year":%d}`, amount, month, year)
}

// TestCreateSavingGoalPeriodGuard covers the month/year comparison. The
// original compared only the month, which rejected January of next year for
// the whole back half of the current one; comparing month and year separately
// then let December of a past year through.
func TestCreateSavingGoalPeriodGuard(t *testing.T) {
	now := time.Now()
	currentMonth := int(now.Month())
	currentYear := now.Year()

	nextMonth := now.AddDate(0, 1, 0)
	previousMonth := now.AddDate(0, -1, 0)

	cases := []struct {
		name       string
		month      int
		year       int
		wantReject bool
	}{
		{"the current month", currentMonth, currentYear, false},
		{"next month", int(nextMonth.Month()), nextMonth.Year(), false},
		{"January of next year", 1, currentYear + 1, false},
		{"December of next year", 12, currentYear + 1, false},
		{"last month", int(previousMonth.Month()), previousMonth.Year(), currentMonth != 1},
		{"December of last year", 12, currentYear - 1, true},
		{"January of last year", 1, currentYear - 1, true},
		{"the current month, several years back", currentMonth, currentYear - 5, true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			status, message := postSavingGoal(t, goalBody(100, testCase.month, testCase.year))

			rejected := status == http.StatusBadRequest

			if rejected != testCase.wantReject {
				t.Errorf("month %d of %d: status %d (%q), want rejected=%v",
					testCase.month, testCase.year, status, message, testCase.wantReject)
			}
		})
	}
}

// TestCreateSavingGoalMonthRange covers the range check, which now runs before
// the period comparison: an out-of-range month was previously measured against
// the current month before anything established it was a month at all.
func TestCreateSavingGoalMonthRange(t *testing.T) {
	currentYear := time.Now().Year()

	for _, month := range []int{-1, 13, 99} {
		status, message := postSavingGoal(t, goalBody(100, month, currentYear+1))

		if status != http.StatusBadRequest {
			t.Errorf("month %d: status %d, want 400", month, status)
		}

		if !strings.Contains(message, "valid month") {
			t.Errorf("month %d: message %q, want the invalid-month message rather than the period one", month, message)
		}
	}
}

// TestCreateSavingGoalRejectsAMalformedBody pins the binding failure path.
func TestCreateSavingGoalRejectsAMalformedBody(t *testing.T) {
	cases := []string{
		`{"amount":0,"month":6,"year":2030}`,
		`{"month":6,"year":2030}`,
		`{"amount":100,"year":2030}`,
		`not json`,
	}

	for _, body := range cases {
		if status, _ := postSavingGoal(t, body); status != http.StatusBadRequest {
			t.Errorf("body %s: status %d, want 400", body, status)
		}
	}
}

// TestGetSavingsDataYearGuard covers the year check that replaced the
// hardcoded 2025.
func TestGetSavingsDataYearGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	currentYear := time.Now().Year()

	cases := []struct {
		year       string
		wantReject bool
	}{
		{fmt.Sprint(currentYear), false},
		{fmt.Sprint(currentYear + 1), false},
		{fmt.Sprint(currentYear - 1), true},
		{"2025", currentYear > 2025},
		{"", true},
		{"not-a-year", true},
	}

	for _, testCase := range cases {
		t.Run(testCase.year, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodGet, "/savings?year="+testCase.year, nil)
			c.Set(services.FinanceIdKey, uint(1))

			dryRunSavingHandler(t).GetSavingsData(c)

			rejected := recorder.Code == http.StatusBadRequest

			if rejected != testCase.wantReject {
				t.Errorf("year %q: status %d, want rejected=%v", testCase.year, recorder.Code, testCase.wantReject)
			}
		})
	}
}
