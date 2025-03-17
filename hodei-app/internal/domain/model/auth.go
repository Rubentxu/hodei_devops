package model

import (
	"errors"
	"time"
)

// Common authentication errors
var (
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrTokenExpired        = errors.New("token expired")
	ErrInvalidToken        = errors.New("invalid token")
	ErrInsufficientRights  = errors.New("insufficient rights")
	ErrUserNotFound        = errors.New("user not found")
	ErrUserAlreadyExists   = errors.New("user already exists")
	ErrPasswordMismatch    = errors.New("password mismatch")
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshTokenExpired = errors.New("refresh token expired")
)

// Credentials represents the authentication credentials
type Credentials struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// Validate validates the credentials
func (c Credentials) Validate() error {
	if c.Username == "" {
		return errors.New("username cannot be empty")
	}
	if c.Password == "" {
		return errors.New("password cannot be empty")
	}
	return nil
}

// TokenDetails contains the JWT token details
type TokenDetails struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	AccessUUID   string `json:"accessUuid,omitempty"`
	RefreshUUID  string `json:"refreshUuid,omitempty"`
	AtExpires    int64  `json:"accessExpires"`
	RtExpires    int64  `json:"refreshExpires"`
}

// TokenMetadata contains the metadata extracted from a token
type TokenMetadata struct {
	UserID       string   `json:"userId"`
	Username     string   `json:"username"`
	AccessUUID   string   `json:"accessUuid"`
	Organization string   `json:"organization,omitempty"`
	Project      string   `json:"project,omitempty"`
	Roles        []string `json:"roles"`
	ExpiresAt    int64    `json:"expiresAt"`
}

// UserAuth represents a user with authentication information
type UserAuth struct {
	User
	Password     string    `json:"-"` // Never expose password in JSON
	RefreshToken string    `json:"-"` // Security-sensitive, don't expose
	LastLogin    time.Time `json:"lastLogin,omitempty"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// NewUserAuth creates a new UserAuth
func NewUserAuth(username, password string, roles []string) *UserAuth {
	return &UserAuth{
		User: User{
			Name:   username,
			Roles:  roles,
			Groups: []string{},
		},
		Password:     password,
		RefreshToken: "",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

// UpdateLastLogin updates the last login time
func (u *UserAuth) UpdateLastLogin() {
	u.LastLogin = time.Now().UTC()
	u.UpdatedAt = time.Now().UTC()
}

// UpdateRefreshToken updates the refresh token
func (u *UserAuth) UpdateRefreshToken(refreshToken string) {
	u.RefreshToken = refreshToken
	u.UpdatedAt = time.Now().UTC()
}
