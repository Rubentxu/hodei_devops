package ports

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"time"
)

// RoleRepository defines operations for managing roles
type RoleRepository interface {
	CreateRole(role model.Role) error
	GetRole(orgID, projectID, roleName string) (*model.Role, error)
	UpdateRole(role model.Role) error
	DeleteRole(orgID, projectID, roleName string) error
	ListRoles(orgID, projectID string) ([]model.Role, error)
}

// GroupRepository defines operations for managing groups
type GroupRepository interface {
	CreateGroup(group model.Group) error
	GetGroup(orgID, projectID, groupName string) (*model.Group, error)
	UpdateGroup(group model.Group) error
	DeleteGroup(orgID, projectID, groupName string) error
	ListGroups(orgID, projectID string) ([]model.Group, error)
}

// UserRepository defines operations for managing users
type UserRepository interface {
	CreateUser(user *model.UserAuth) error
	GetUserByID(id string) (*model.UserAuth, error)
	GetUserByUsername(username string) (*model.UserAuth, error)
	UpdateUser(user *model.UserAuth) error
	DeleteUser(id string) error
	ListUsers() ([]*model.UserAuth, error)
}

// TokenRepository defines operations for managing tokens
type TokenRepository interface {
	StoreAccessToken(ctx context.Context, userID, accessUUID string, expiry time.Duration) error
	StoreRefreshToken(ctx context.Context, userID, refreshUUID string, expiry time.Duration) error
	DeleteAccessToken(ctx context.Context, accessUUID string) error
	DeleteRefreshToken(ctx context.Context, refreshUUID string) error
	DeleteUserTokens(ctx context.Context, userID string) error
	ValidateAccessToken(ctx context.Context, accessUUID string) (string, error)
	ValidateRefreshToken(ctx context.Context, refreshUUID string) (string, error)
}

// PasswordHasher defines operations for password hashing
type PasswordHasher interface {
	HashPassword(password string) (string, error)
	CheckPasswordHash(password, hash string) bool
}

// TokenService defines operations for JWT token management
type TokenService interface {
	GenerateTokens(userID, username, organization, project string, roles []string) (*model.TokenDetails, error)
	ValidateAccessToken(tokenString string) (*model.TokenMetadata, error)
	ValidateRefreshToken(tokenString string) (string, string, error)
}

// AuthService defines operations for authentication
type AuthService interface {
	Register(ctx context.Context, username, password, organization, project string, roles []string) (*model.UserAuth, error)
	Login(ctx context.Context, credentials model.Credentials) (*model.TokenDetails, error)
	Logout(ctx context.Context, accessUUID string) error
	RefreshToken(ctx context.Context, refreshToken string) (*model.TokenDetails, error)
	ValidateToken(ctx context.Context, tokenString string) (*model.TokenMetadata, error)
}
