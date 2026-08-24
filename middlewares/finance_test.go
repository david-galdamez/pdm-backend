package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"pdm-backend/services"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeChecker answers from a fixed set of (user, finance) pairs so the
// middleware can be exercised without a database.
type fakeChecker struct {
	allowed map[[2]uint]bool
	err     error
}

func (f fakeChecker) CanAccessFinance(userId, financeId uint) (bool, error) {
	if f.err != nil {
		return false, f.err
	}

	return f.allowed[[2]uint{userId, financeId}], nil
}

// run drives one request through the middleware with claims already in place,
// as AuthMiddleware would have left them. It reports the status and the finance
// id the handler saw, or 0 when the request never reached the handler.
func run(middleware gin.HandlerFunc, target string, claims *services.JWTClaims) (int, uint) {
	gin.SetMode(gin.TestMode)

	var seen uint

	r := gin.New()
	r.GET("/finances/:id", func(c *gin.Context) {
		if claims != nil {
			c.Set("claims", claims)
		}
		c.Next()
	}, middleware, func(c *gin.Context) {
		seen = services.FinanceId(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))

	return w.Code, seen
}

func TestFinanceAccess(t *testing.T) {
	claims := &services.JWTClaims{UserID: 1, FinanceID: 10}

	checker := fakeChecker{allowed: map[[2]uint]bool{
		{1, 10}: true, // the caller's own personal finance
		{1, 42}: true, // a shared finance they belong to
	}}

	tests := []struct {
		name       string
		target     string
		claims     *services.JWTClaims
		checker    fakeChecker
		wantStatus int
		wantID     uint
	}{
		{
			name:       "falls back to the personal finance",
			target:     "/finances/1",
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusOK,
			wantID:     10,
		},
		{
			name:       "allows a shared finance the caller belongs to",
			target:     "/finances/1?finance_id=42",
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusOK,
			wantID:     42,
		},
		{
			name:       "denies a finance the caller does not belong to",
			target:     "/finances/1?finance_id=99",
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "rejects a non-numeric finance id",
			target:     "/finances/1?finance_id=abc",
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "rejects a zero finance id",
			target:     "/finances/1?finance_id=0",
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "fails closed when the lookup errors",
			target:     "/finances/1?finance_id=42",
			claims:     claims,
			checker:    fakeChecker{err: errors.New("connection refused")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "rejects a request with no claims",
			target:     "/finances/1?finance_id=42",
			claims:     nil,
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, financeId := run(FinanceAccess(test.checker), test.target, test.claims)

			if status != test.wantStatus {
				t.Errorf("status = %d, want %d", status, test.wantStatus)
			}

			if financeId != test.wantID {
				t.Errorf("finance id = %d, want %d", financeId, test.wantID)
			}
		})
	}
}

func TestFinanceAccessFromParam(t *testing.T) {
	claims := &services.JWTClaims{UserID: 1, FinanceID: 10}

	checker := fakeChecker{allowed: map[[2]uint]bool{{1, 42}: true}}

	tests := []struct {
		name       string
		target     string
		wantStatus int
		wantID     uint
	}{
		{
			name:       "allows a finance the caller belongs to",
			target:     "/finances/42",
			wantStatus: http.StatusOK,
			wantID:     42,
		},
		{
			name:       "denies a finance the caller does not belong to",
			target:     "/finances/99",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "ignores the query parameter entirely",
			target:     "/finances/99?finance_id=42",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "rejects a non-numeric path id",
			target:     "/finances/abc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, financeId := run(FinanceAccessFromParam(checker, "id"), test.target, claims)

			if status != test.wantStatus {
				t.Errorf("status = %d, want %d", status, test.wantStatus)
			}

			if financeId != test.wantID {
				t.Errorf("finance id = %d, want %d", financeId, test.wantID)
			}
		})
	}
}
