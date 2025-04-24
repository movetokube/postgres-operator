package utils

import cryptorand "crypto/rand"
import "math/rand"
import "math/big"

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func GetRandomString(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// If the secure random number generator malfunctions it will return an error
func GetSecureRandomString(length int) (string, error) {
	b := make([]rune, length)
	for i := 0; i < length; i++ {
		num, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(letterRunes))))
		if err != nil {
			return "", err
		}
		b[i] = letterRunes[num.Int64()]
	}

	return string(b), nil
}
