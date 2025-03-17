package iam

import (
	"context"

	"time"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// AuthenticationService implements the AuthService interface
type AuthenticationService struct {
	userRepo     ports.UserRepository
	tokenRepo    ports.TokenRepository
	tokenService ports.TokenService
	pwdHasher    ports.PasswordHasher
}

// NewAuthenticationService creates a new AuthenticationService
func NewAuthenticationService(
	userRepo ports.UserRepository,
	tokenRepo ports.TokenRepository,
	tokenService ports.TokenService,
	pwdHasher ports.PasswordHasher,
) ports.AuthService {
	return &AuthenticationService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		tokenService: tokenService,
		pwdHasher:    pwdHasher,
	}
}

// Register registers a new user
func (s *AuthenticationService) Register(
	ctx context.Context,
	username, password, organization, project string,
	roles []string,
) (*model.UserAuth, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetUserByUsername(username)
	if err == nil && existingUser != nil {
		return nil, model.ErrUserAlreadyExists
	}

	// Hash the password
	hashedPassword, err := s.pwdHasher.HashPassword(password)
	if err != nil {
		return nil, err
	}

	// Create new user
	user := model.NewUserAuth(username, hashedPassword, roles)

	// Save user to repository
	if err := s.userRepo.CreateUser(user); err != nil {
		return nil, err
	}

	return user, nil
}

// Login authenticates a user and returns token details
func (s *AuthenticationService) Login(ctx context.Context, credentials model.Credentials) (*model.TokenDetails, error) {
	// Validate credentials
	if err := credentials.Validate(); err != nil {
		return nil, err
	}

	// Get user by username
	user, err := s.userRepo.GetUserByUsername(credentials.Username)
	if err != nil {
		return nil, model.ErrInvalidCredentials
	}

	// Check password
	if !s.pwdHasher.CheckPasswordHash(credentials.Password, user.Password) {
		return nil, model.ErrInvalidCredentials
	}

	// Generate tokens
	td, err := s.tokenService.GenerateTokens(
		user.ID.String(),
		user.Name,
		"", // Organization - assuming this is not set during login
		"", // Project - assuming this is not set during login
		user.Roles,
	)
	if err != nil {
		return nil, err
	}

	// Store tokens in repository
	accessExpiry := time.Unix(td.AtExpires, 0).Sub(time.Now())
	refreshExpiry := time.Unix(td.RtExpires, 0).Sub(time.Now())

	if err := s.tokenRepo.StoreAccessToken(ctx, user.ID.String(), td.AccessUUID, accessExpiry); err != nil {
		return nil, err
	}

	if err := s.tokenRepo.StoreRefreshToken(ctx, user.ID.String(), td.RefreshUUID, refreshExpiry); err != nil {
		// Clean up access token if refresh token storage fails
		_ = s.tokenRepo.DeleteAccessToken(ctx, td.AccessUUID)
		return nil, err
	}

	// Update user's last login time
	user.UpdateLastLogin()
	user.UpdateRefreshToken(td.RefreshToken)
	if err := s.userRepo.UpdateUser(user); err != nil {
		// Non-critical error, just log it in a real implementation
		// log.Printf("Failed to update user's last login time: %v", err)
	}

	return td, nil
}

// Logout invalidates a user's access token
func (s *AuthenticationService) Logout(ctx context.Context, accessUUID string) error {
	return s.tokenRepo.DeleteAccessToken(ctx, accessUUID)
}

// RefreshToken refreshes an access token using a refresh token
func (s *AuthenticationService) RefreshToken(ctx context.Context, refreshToken string) (*model.TokenDetails, error) {
	// Validate refresh token
	userID, refreshUUID, err := s.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	// Check if refresh token exists in repository
	storedUserID, err := s.tokenRepo.ValidateRefreshToken(ctx, refreshUUID)
	if err != nil {
		return nil, err
	}

	// Verify user IDs match
	if storedUserID != userID {
		return nil, model.ErrInvalidRefreshToken
	}

	// Get user details
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// Delete old refresh token
	if err := s.tokenRepo.DeleteRefreshToken(ctx, refreshUUID); err != nil {
		return nil, err
	}

	// Generate new tokens
	td, err := s.tokenService.GenerateTokens(
		user.ID.String(),
		user.Name,
		"", // Organization - assuming this is not set during token refresh
		"", // Project - assuming this is not set during token refresh
		user.Roles,
	)
	if err != nil {
		return nil, err
	}

	// Store new tokens
	accessExpiry := time.Unix(td.AtExpires, 0).Sub(time.Now())
	refreshExpiry := time.Unix(td.RtExpires, 0).Sub(time.Now())

	if err := s.tokenRepo.StoreAccessToken(ctx, user.ID.String(), td.AccessUUID, accessExpiry); err != nil {
		return nil, err
	}

	if err := s.tokenRepo.StoreRefreshToken(ctx, user.ID.String(), td.RefreshUUID, refreshExpiry); err != nil {
		// Clean up access token if refresh token storage fails
		_ = s.tokenRepo.DeleteAccessToken(ctx, td.AccessUUID)
		return nil, err
	}

	// Update user's refresh token
	user.UpdateRefreshToken(td.RefreshToken)
	if err := s.userRepo.UpdateUser(user); err != nil {
		// Non-critical error, just log it in a real implementation
		// log.Printf("Failed to update user's refresh token: %v", err)
	}

	return td, nil
}

// ValidateToken validates an access token and returns token metadata
func (s *AuthenticationService) ValidateToken(ctx context.Context, tokenString string) (*model.TokenMetadata, error) {
	// Parse and validate token
	metadata, err := s.tokenService.ValidateAccessToken(tokenString)
	if err != nil {
		return nil, err
	}

	// Check if token exists in repository
	userID, err := s.tokenRepo.ValidateAccessToken(ctx, metadata.AccessUUID)
	if err != nil {
		return nil, model.ErrInvalidToken
	}

	// Verify user IDs match
	if userID != metadata.UserID {
		return nil, model.ErrInvalidToken
	}

	return metadata, nil
}
