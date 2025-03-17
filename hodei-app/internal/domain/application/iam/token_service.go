package iam

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"github.com/google/uuid"
)

// JWTTokenService implements the TokenService interface
type JWTTokenService struct {
	accessSecret  string
	refreshSecret string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
}

// NewJWTTokenService creates a new JWTTokenService
func NewJWTTokenService(accessSecret, refreshSecret string, accessExpiry, refreshExpiry time.Duration) ports.TokenService {
	return &JWTTokenService{
		accessSecret:  accessSecret,
		refreshSecret: refreshSecret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
	}
}

// GenerateTokens generates JWT access and refresh tokens
func (s *JWTTokenService) GenerateTokens(userID, username, organization, project string, roles []string) (*model.TokenDetails, error) {
	td := &model.TokenDetails{
		AtExpires:   time.Now().Add(s.accessExpiry).Unix(),
		AccessUUID:  uuid.New().String(),
		RtExpires:   time.Now().Add(s.refreshExpiry).Unix(),
		RefreshUUID: uuid.New().String(),
	}

	// Create Access Token
	atClaims := jwt.MapClaims{
		"user_id":      userID,
		"username":     username,
		"access_uuid":  td.AccessUUID,
		"organization": organization,
		"project":      project,
		"roles":        roles,
		"exp":          td.AtExpires,
	}
	at := jwt.NewWithClaims(jwt.SigningMethodHS256, atClaims)
	accessToken, err := at.SignedString([]byte(s.accessSecret))
	if err != nil {
		return nil, err
	}
	td.AccessToken = accessToken

	// Create Refresh Token
	rtClaims := jwt.MapClaims{
		"user_id":      userID,
		"refresh_uuid": td.RefreshUUID,
		"exp":          td.RtExpires,
	}
	rt := jwt.NewWithClaims(jwt.SigningMethodHS256, rtClaims)
	refreshToken, err := rt.SignedString([]byte(s.refreshSecret))
	if err != nil {
		return nil, err
	}
	td.RefreshToken = refreshToken

	return td, nil
}

// ValidateAccessToken validates the access token and returns the token metadata
func (s *JWTTokenService) ValidateAccessToken(tokenString string) (*model.TokenMetadata, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.accessSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, model.ErrTokenExpired
		}
		return nil, model.ErrInvalidToken
	}

	if !token.Valid {
		return nil, model.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	// Extract token metadata
	userID, ok := claims["user_id"].(string)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	username, ok := claims["username"].(string)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	accessUUID, ok := claims["access_uuid"].(string)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	organization, ok := claims["organization"].(string)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	project, ok := claims["project"].(string)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	// Extract roles
	rolesInterface, ok := claims["roles"].([]interface{})
	if !ok {
		return nil, model.ErrInvalidToken
	}

	roles := make([]string, len(rolesInterface))
	for i, role := range rolesInterface {
		roles[i], ok = role.(string)
		if !ok {
			return nil, model.ErrInvalidToken
		}
	}

	expiresAt, ok := claims["exp"].(float64)
	if !ok {
		return nil, model.ErrInvalidToken
	}

	return &model.TokenMetadata{
		UserID:       userID,
		Username:     username,
		AccessUUID:   accessUUID,
		Organization: organization,
		Project:      project,
		Roles:        roles,
		ExpiresAt:    int64(expiresAt),
	}, nil
}

// ValidateRefreshToken validates the refresh token and returns the user ID and refresh UUID
func (s *JWTTokenService) ValidateRefreshToken(tokenString string) (string, string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.refreshSecret), nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", "", model.ErrRefreshTokenExpired
		}
		return "", "", model.ErrInvalidRefreshToken
	}

	if !token.Valid {
		return "", "", model.ErrInvalidRefreshToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", "", model.ErrInvalidRefreshToken
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", "", model.ErrInvalidRefreshToken
	}

	refreshUUID, ok := claims["refresh_uuid"].(string)
	if !ok {
		return "", "", model.ErrInvalidRefreshToken
	}

	return userID, refreshUUID, nil
}
