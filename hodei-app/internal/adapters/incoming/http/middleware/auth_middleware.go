package middleware

import (
	"context"
	"net/http"
	"strings"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// AuthMiddleware handles JWT authentication for HTTP requests
type AuthMiddleware struct {
	authService ports.AuthService
}

// NewAuthMiddleware creates a new AuthMiddleware
func NewAuthMiddleware(authService ports.AuthService) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
	}
}

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

// Context keys for storing authentication data
const (
	UserIDKey        contextKey = "user_id"
	UsernameKey      contextKey = "username"
	RolesKey         contextKey = "roles"
	OrganizationKey  contextKey = "organization"
	ProjectKey       contextKey = "project"
	TokenMetadataKey contextKey = "token_metadata"
)

// Middleware returns an HTTP middleware function for JWT authentication
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check if the header has the Bearer prefix
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Validate token
		metadata, err := m.authService.ValidateToken(r.Context(), tokenString)
		if err != nil {
			switch err {
			case model.ErrTokenExpired:
				http.Error(w, "Token expired", http.StatusUnauthorized)
			case model.ErrInvalidToken:
				http.Error(w, "Invalid token", http.StatusUnauthorized)
			default:
				http.Error(w, "Authentication failed", http.StatusUnauthorized)
			}
			return
		}

		// Store token metadata in request context
		ctx := context.WithValue(r.Context(), UserIDKey, metadata.UserID)
		ctx = context.WithValue(ctx, UsernameKey, metadata.Username)
		ctx = context.WithValue(ctx, RolesKey, metadata.Roles)
		ctx = context.WithValue(ctx, OrganizationKey, metadata.Organization)
		ctx = context.WithValue(ctx, ProjectKey, metadata.Project)
		ctx = context.WithValue(ctx, TokenMetadataKey, metadata)

		// Call the next handler with the updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole returns a middleware that checks if the user has a specific role
func (m *AuthMiddleware) RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get roles from context
			rolesInterface := r.Context().Value(RolesKey)
			if rolesInterface == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			roles, ok := rolesInterface.([]string)
			if !ok {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			// Check if the user has the required role
			hasRole := false
			for _, r := range roles {
				if r == role {
					hasRole = true
					break
				}
			}

			if !hasRole {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			// Call the next handler
			next.ServeHTTP(w, r)
		})
	}
}

// GetTokenMetadata extracts the token metadata from the request context
func GetTokenMetadata(ctx context.Context) (*model.TokenMetadata, bool) {
	metadataInterface := ctx.Value(TokenMetadataKey)
	if metadataInterface == nil {
		return nil, false
	}

	metadata, ok := metadataInterface.(*model.TokenMetadata)
	return metadata, ok
}

// GetUserID extracts the user ID from the request context
func GetUserID(ctx context.Context) (string, bool) {
	userIDInterface := ctx.Value(UserIDKey)
	if userIDInterface == nil {
		return "", false
	}

	userID, ok := userIDInterface.(string)
	return userID, ok
}

// GetUsername extracts the username from the request context
func GetUsername(ctx context.Context) (string, bool) {
	usernameInterface := ctx.Value(UsernameKey)
	if usernameInterface == nil {
		return "", false
	}

	username, ok := usernameInterface.(string)
	return username, ok
}

// GetRoles extracts the roles from the request context
func GetRoles(ctx context.Context) ([]string, bool) {
	rolesInterface := ctx.Value(RolesKey)
	if rolesInterface == nil {
		return nil, false
	}

	roles, ok := rolesInterface.([]string)
	return roles, ok
}

// GetOrganization extracts the organization from the request context
func GetOrganization(ctx context.Context) (string, bool) {
	orgInterface := ctx.Value(OrganizationKey)
	if orgInterface == nil {
		return "", false
	}

	org, ok := orgInterface.(string)
	return org, ok
}

// GetProject extracts the project from the request context
func GetProject(ctx context.Context) (string, bool) {
	projectInterface := ctx.Value(ProjectKey)
	if projectInterface == nil {
		return "", false
	}

	project, ok := projectInterface.(string)
	return project, ok
}
