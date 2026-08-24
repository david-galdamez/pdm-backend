package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"pdm-backend/services"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeAdminChecker answers from a fixed set of (finance, user) pairs so the
// middleware can be exercised without a database.
type fakeAdminChecker struct {
	admins map[[2]uint]bool
	err    error
}

func (f fakeAdminChecker) IsFinanceAdmin(financeId, userId uint) (bool, error) {
	if f.err != nil {
		return false, f.err
	}

	return f.admins[[2]uint{financeId, userId}], nil
}

// runAdmin drives a request through RequireFinanceAdmin as if a prior
// FinanceAccess middleware had already resolved and stashed the finance id.
func runAdmin(checker FinanceAdminChecker, financeId uint, claims *services.JWTClaims) int {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/finances/:id", func(c *gin.Context) {
		if claims != nil {
			c.Set("claims", claims)
		}
		c.Set(services.FinanceIdKey, financeId)
		c.Next()
	}, RequireFinanceAdmin(checker), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/finances/1", nil))

	return w.Code
}

func TestRequireFinanceAdmin(t *testing.T) {
	claims := &services.JWTClaims{UserID: 1}

	checker := fakeAdminChecker{admins: map[[2]uint]bool{{42, 1}: true}}

	tests := []struct {
		name       string
		financeId  uint
		claims     *services.JWTClaims
		checker    FinanceAdminChecker
		wantStatus int
	}{
		{
			name:       "allows the finance's admin",
			financeId:  42,
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusOK,
		},
		{
			name:       "denies a member who is not the admin",
			financeId:  99,
			claims:     claims,
			checker:    checker,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "fails closed when the lookup errors",
			financeId:  42,
			claims:     claims,
			checker:    fakeAdminChecker{err: errors.New("connection refused")},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "rejects a request with no claims",
			financeId:  42,
			claims:     nil,
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := runAdmin(test.checker, test.financeId, test.claims)

			if status != test.wantStatus {
				t.Errorf("status = %d, want %d", status, test.wantStatus)
			}
		})
	}
}
