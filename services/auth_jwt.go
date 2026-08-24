package services

import (
	"fmt"
	"log"
	"os"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
)

var secret string

type JWTClaims struct {
	UserID    uint   `json:"userId"`
	UserName  string `json:"userName"`
	UserEmail string `json:"userEmail"`
	FinanceID uint   `json:"financeId"`
	SavingsID uint   `json:"savingsId"`
	jwt.RegisteredClaims
}

func init() {

	if os.Getenv("ENV") != "production" {
		err := godotenv.Load(".env")
		if err != nil {
			log.Println("Could not load .env (this is expected in production)")
		}
	}

	secret = os.Getenv("SECRET_WORD")
	if secret == "" {
		log.Fatal("SECRET_WORD is not set")
	}
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

	return token.SignedString([]byte(secret))
}

func ValidateJWT(tokenString string) (*jwt.Token, *JWTClaims, error) {
	claims := &JWTClaims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("invalid signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, nil, err
	}

	return token, claims, nil
}
