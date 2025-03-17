package iam

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"golang.org/x/crypto/bcrypt"
)

// BcryptPasswordHasher implements the PasswordHasher interface using bcrypt
type BcryptPasswordHasher struct {
	cost int
}

// NewBcryptPasswordHasher creates a new BcryptPasswordHasher
func NewBcryptPasswordHasher(cost int) ports.PasswordHasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &BcryptPasswordHasher{cost: cost}
}

// HashPassword hashes a password using bcrypt
func (h *BcryptPasswordHasher) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPasswordHash checks if a password matches a hash
func (h *BcryptPasswordHasher) CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
