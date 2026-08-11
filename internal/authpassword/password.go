package authpassword

import (
	"errors"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var ErrPasswordRequired = errors.New("password is required")

// This is only used to keep the unknown-login comparison on the same bcrypt
// path as a known identity. It is not a credential and is never returned.
const dummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func Hash(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrPasswordRequired
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func Verify(hash string, password string) bool {
	if strings.TrimSpace(hash) == "" {
		hash = dummyPasswordHash
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
