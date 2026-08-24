package services

import (
	"crypto/rand"
	"math/big"
)

const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func GenerateInvitationCode(length int) (string, error) {
	code := make([]byte, length)

	for i := range code {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(codeAlphabet))))
		if err != nil {
			return "", err
		}
		code[i] = codeAlphabet[num.Int64()]
	}

	return string(code), nil
}
