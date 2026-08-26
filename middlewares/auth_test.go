package middlewares

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"pdm-backend/services"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// testJWTSecret is what config.Get().JWT_SECRET resolves to for this test
// binary: config caches it behind a sync.Once, so it must be set before
// anything in the package calls services.ValidateJWT for the first time.
const testJWTSecret = "test-only-secret-not-used-anywhere-else"

func TestMain(m *testing.M) {
	os.Setenv("ENV", "test")
	os.Setenv("JWT_SECRET", testJWTSecret)

	os.Exit(m.Run())
}

// fakeVersionChecker answers GetTokenVersion from a fixed map, so the
// middleware can be exercised without a database.
type fakeVersionChecker struct {
	versions map[uint]uint
	err      error
}

func (f fakeVersionChecker) GetTokenVersion(userId uint) (uint, error) {
	if f.err != nil {
		return 0, f.err
	}

	return f.versions[userId], nil
}

func runAuth(t *testing.T, checker TokenVersionChecker, authHeader string) (int, bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var claimsSet bool

	r := gin.New()
	r.GET("/protected", AuthMiddleware(checker), func(c *gin.Context) {
		_, claimsSet = c.Get("claims")
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	return w.Code, claimsSet
}

func signToken(t *testing.T, claims services.JWTClaims) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Must match the JWT_SECRET the test process's config.Get() resolves to.
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}

	return signed
}

func TestAuthMiddleware(t *testing.T) {
	now := time.Now()

	validClaims := services.JWTClaims{
		UserID:       1,
		TokenVersion: 2,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	expiredClaims := services.JWTClaims{
		UserID:       1,
		TokenVersion: 2,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	}

	noExpiryClaims := services.JWTClaims{
		UserID:       1,
		TokenVersion: 2,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt: jwt.NewNumericDate(now),
			// ExpiresAt deliberately left unset, as every pre-fix token was.
		},
	}

	staleVersionClaims := services.JWTClaims{
		UserID:       1,
		TokenVersion: 1, // one behind the current version below
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}

	checker := fakeVersionChecker{versions: map[uint]uint{1: 2}}

	tests := []struct {
		name          string
		authHeader    string
		checker       TokenVersionChecker
		wantStatus    int
		wantClaimsSet bool
	}{
		{
			name:          "accepts a current, unexpired token",
			authHeader:    "Bearer " + signToken(t, validClaims),
			checker:       checker,
			wantStatus:    http.StatusOK,
			wantClaimsSet: true,
		},
		{
			name:       "rejects an expired token",
			authHeader: "Bearer " + signToken(t, expiredClaims),
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rejects a token with no expiry at all",
			authHeader: "Bearer " + signToken(t, noExpiryClaims),
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rejects a token whose version is behind the user's current one",
			authHeader: "Bearer " + signToken(t, staleVersionClaims),
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "fails closed when the version lookup errors",
			authHeader: "Bearer " + signToken(t, validClaims),
			checker:    fakeVersionChecker{err: errors.New("connection refused")},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rejects a missing Authorization header",
			authHeader: "",
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "rejects a header with no Bearer prefix",
			authHeader: signToken(t, validClaims),
			checker:    checker,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, claimsSet := runAuth(t, test.checker, test.authHeader)

			if status != test.wantStatus {
				t.Errorf("status = %d, want %d", status, test.wantStatus)
			}

			if claimsSet != test.wantClaimsSet {
				t.Errorf("claims set = %v, want %v", claimsSet, test.wantClaimsSet)
			}
		})
	}
}
