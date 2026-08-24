package services

import (
	"fmt"
	"pdm-backend/internal/config"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// accessTokenTTL is deliberately short-lived rather than tied to a refresh
// token: re-login after expiry is an acceptable cost for this app, and it
// keeps the change entirely server-side.
const accessTokenTTL = 7 * 24 * time.Hour

type JWTClaims struct {
	UserID    uint   `json:"userId"`
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
	FinanceID uint   `json:"financeId"`
	SavingsID uint   `json:"savingsId"`
	// TokenVersion must match the user's current token_version or the token is
	// treated as revoked, even if it has not expired yet.
	TokenVersion uint `json:"tokenVersion"`
	jwt.RegisteredClaims
}

func GenerateJWT(userId uint, userName string, userEmail string, financeId, savingsId, tokenVersion uint) (string, error) {

	now := time.Now()

	claims := JWTClaims{
		UserID:       userId,
		UserName:     userName,
		UserEmail:    userEmail,
		FinanceID:    financeId,
		SavingsID:    savingsId,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(accessTokenTTL)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString([]byte(config.Get().JWT_SECRET))
}

func ValidateJWT(tokenString string) (*jwt.Token, *JWTClaims, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method: %v", token.Header["alg"])
		}
		return []byte(config.Get().JWT_SECRET), nil
	}, jwt.WithExpirationRequired())

	if err != nil {
		return nil, nil, err
	}

	return token, claims, nil
}
