package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	randomString := hex.EncodeToString(bytes)
	return randomString[:length], nil
}
