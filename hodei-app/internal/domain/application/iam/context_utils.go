package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
)

// contextKey is a private type for context keys used by the auth package
type contextKey string

const (
	// userIDKey is the context key for user ID
	userIDKey contextKey = "user_id"
	// usernameKey is the context key for username
	usernameKey contextKey = "username"
	// userRolesKey is the context key for user roles
	userRolesKey contextKey = "user_roles"
	// userGroupsKey is the context key for user groups
	userGroupsKey contextKey = "user_groups"
)

// GetUserID extracts the user ID from the context
func GetUserID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(userIDKey).(string)
	return id, ok
}

// GetUsername extracts the username from the context
func GetUsername(ctx context.Context) (string, bool) {
	username, ok := ctx.Value(usernameKey).(string)
	return username, ok
}

// GetUserRoles extracts the user roles from the context
func GetUserRoles(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(userRolesKey).([]string)
	return roles, ok
}

// GetUserGroups extracts the user groups from the context
func GetUserGroups(ctx context.Context) ([]string, bool) {
	groups, ok := ctx.Value(userGroupsKey).([]string)
	return groups, ok
}

// SetUserID sets the user ID in the context
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// SetUsername sets the username in the context
func SetUsername(ctx context.Context, username string) context.Context {
	return context.WithValue(ctx, usernameKey, username)
}

// SetUserRoles sets the user roles in the context
func SetUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, userRolesKey, roles)
}

// SetUserGroups sets the user groups in the context
func SetUserGroups(ctx context.Context, groups []string) context.Context {
	return context.WithValue(ctx, userGroupsKey, groups)
}

// SetUserContext sets all user-related information in the context
func SetUserContext(ctx context.Context, user *model.User) context.Context {
	ctx = SetUserID(ctx, user.ID.String())
	ctx = SetUsername(ctx, user.Name) // User model has Name field, not Username
	ctx = SetUserRoles(ctx, user.Roles)
	ctx = SetUserGroups(ctx, user.Groups)
	return ctx
}
