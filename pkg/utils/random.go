package utils

import (
	cryptorand "crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"strings"
)

var (
	letterRunes  = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")
	lowerRunes   = []rune("abcdefghijklmnopqrstuvwxyz")
	upperRunes   = []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	numericRunes = []rune("0123456789")
	specialRunes = []rune("!@#$%^&*")
)

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

type PostgresPassPolicy struct {
	Length            int
	MinLower          int
	MinUpper          int
	MinNumeric        int
	MinSpecial        int
	ExcludeChars      string
	EnsureFirstLetter bool
}

// GeneratePassword creates a random password based on the provided configuration.
// It ensures that the generated password meets the length and complexity requirements
// defined in PostgresPassPolicy.
func GeneratePassword(config PostgresPassPolicy) (string, error) {
	if config.Length == 0 {
		config.Length = 15 // Default length
	}

	if err := validatePasswordConfig(config); err != nil {
		return "", err
	}

	var password []rune

	chars, err := collectRequiredChars(config)
	if err != nil {
		return "", err
	}
	password = append(password, chars...)

	chars, err = fillRemainingChars(config, len(password))
	if err != nil {
		return "", err
	}
	password = append(password, chars...)

	shuffleRunes(password)

	if config.EnsureFirstLetter {
		if err := ensureFirstCharIsLetter(password); err != nil {
			return "", err
		}
	}

	return string(password), nil
}

func validatePasswordConfig(config PostgresPassPolicy) error {
	requiredLength := config.MinLower + config.MinUpper + config.MinNumeric + config.MinSpecial
	if config.Length < requiredLength {
		return fmt.Errorf("password length %d is less than required minimum characters %d", config.Length, requiredLength)
	}
	return nil
}

func collectRequiredChars(config PostgresPassPolicy) ([]rune, error) {
	var password []rune

	categories := []struct {
		source []rune
		count  int
	}{
		{lowerRunes, config.MinLower},
		{upperRunes, config.MinUpper},
		{numericRunes, config.MinNumeric},
		{specialRunes, config.MinSpecial},
	}

	for _, cat := range categories {
		if cat.count > 0 {
			chars, err := pickRandomChars(cat.source, cat.count, config.ExcludeChars)
			if err != nil {
				return nil, err
			}
			password = append(password, chars...)
		}
	}
	return password, nil
}

func fillRemainingChars(config PostgresPassPolicy, currentLength int) ([]rune, error) {
	remaining := config.Length - currentLength
	if remaining <= 0 {
		return nil, nil
	}

	// Default pool is Alphanumeric (Legacy behavior)
	pool := append([]rune(nil), letterRunes...)

	// If Special characters are required (Min > 0), add them to the pool for the remaining characters too
	if config.MinSpecial > 0 {
		pool = append(pool, specialRunes...)
	}

	return pickRandomChars(pool, remaining, config.ExcludeChars)
}

func pickRandomChars(source []rune, count int, exclude string) ([]rune, error) {
	filtered := filterRunes(source, exclude)
	if len(filtered) == 0 && count > 0 {
		return nil, errors.New("no characters available for required category after exclusion")
	}

	res := make([]rune, count)
	for i := 0; i < count; i++ {
		num, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(filtered))))
		if err != nil {
			return nil, err
		}
		res[i] = filtered[num.Int64()]
	}
	return res, nil
}

func shuffleRunes(runes []rune) {
	rand.Shuffle(len(runes), func(i, j int) {
		runes[i], runes[j] = runes[j], runes[i]
	})
}

func ensureFirstCharIsLetter(password []rune) error {
	if len(password) == 0 {
		return nil
	}

	isLetter := func(r rune) bool {
		return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
	}

	if isLetter(password[0]) {
		return nil
	}

	for i := 1; i < len(password); i++ {
		if isLetter(password[i]) {
			password[0], password[i] = password[i], password[0]
			return nil
		}
	}

	return errors.New("cannot ensure first letter: no letters in generated password")
}

func filterRunes(runes []rune, exclude string) []rune {
	if exclude == "" {
		return runes
	}
	var res []rune
	for _, r := range runes {
		if !strings.ContainsRune(exclude, r) {
			res = append(res, r)
		}
	}
	return res
}
