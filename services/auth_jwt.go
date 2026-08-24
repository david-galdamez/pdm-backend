package services

import (
	"fmt"
	"pdm-backend/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID    uint   `json:"userId"`
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
	FinanceID uint   `json:"financeId"`
	SavingsID uint   `json:"savingsId"`
	jwt.RegisteredClaims
}

func GenerateJWT(userId uint, userName string, userEmail string, financeId, savingsId uint) (string, error) {

	claims := JWTClaims{
		UserID:    userId,
		UserName:  userName,
		UserEmail: userEmail,
		FinanceID: financeId,
		SavingsID: savingsId,
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
	})

	if err != nil {
		return nil, nil, err
	}

	return token, claims, nil
}
