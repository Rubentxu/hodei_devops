package iam

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"sync"
	"time"
)

// IdentityService manages users and service accounts
type IdentityService struct {
	userRepo ports.UserRepository
	// Service accounts are still managed in memory until we create a repository for them
	mu              sync.RWMutex
	serviceAccounts map[model.AggregateID]*model.ServiceAccount
}

// NewIdentityService creates a new instance of IdentityService
func NewIdentityService(userRepo ports.UserRepository) *IdentityService {
	return &IdentityService{
		userRepo:        userRepo,
		serviceAccounts: make(map[model.AggregateID]*model.ServiceAccount),
	}
}

// CreateUser creates a new user
func (is *IdentityService) CreateUser(user *model.User) error {
	// Convert User to UserAuth
	userAuth := &model.UserAuth{
		User:         *user,
		Password:     "", // Password should be set separately with proper hashing
		LastLogin:    time.Time{}, // Zero value for time.Time
		RefreshToken: "",
	}
	return is.userRepo.CreateUser(userAuth)
}

// GetUser retrieves a user by ID
func (is *IdentityService) GetUser(userID model.AggregateID) (*model.User, error) {
	userAuth, err := is.userRepo.GetUserByID(userID.String())
	if err != nil {
		return nil, err
	}
	return &userAuth.User, nil
}

// GetUserByUsername retrieves a user by username
func (is *IdentityService) GetUserByUsername(username string) (*model.User, error) {
	userAuth, err := is.userRepo.GetUserByUsername(username)
	if err != nil {
		return nil, err
	}
	return &userAuth.User, nil
}

// UpdateUser updates an existing user
func (is *IdentityService) UpdateUser(user *model.User) error {
	// First get the existing user to preserve auth-related fields
	existingUser, err := is.userRepo.GetUserByID(user.ID.String())
	if err != nil {
		return err
	}
	
	// Update the User part while preserving auth-related fields
	existingUser.User = *user
	
	return is.userRepo.UpdateUser(existingUser)
}

// DeleteUser deletes a user
func (is *IdentityService) DeleteUser(userID model.AggregateID) error {
	return is.userRepo.DeleteUser(userID.String())
}

// ListUsers returns all users
func (is *IdentityService) ListUsers() ([]*model.User, error) {
	userAuths, err := is.userRepo.ListUsers()
	if err != nil {
		return nil, err
	}
	
	// Convert from []*model.UserAuth to []*model.User
	users := make([]*model.User, len(userAuths))
	for i, userAuth := range userAuths {
		userCopy := userAuth.User
		users[i] = &userCopy
	}
	
	return users, nil
}

// AssignRole assigns a role to a user or service account
func (is *IdentityService) AssignRole(subject model.Subject, roleName string) error {
	switch s := subject.(type) {
	case *model.User:
		// Check if the role is already assigned
		for _, role := range s.Roles {
			if role == roleName {
				return nil // Role already assigned
			}
		}
		
		// Add the role
		s.Roles = append(s.Roles, roleName)
		
		// Update the user
		return is.UpdateUser(s)
	case *model.ServiceAccount:
		// Check if the role is already assigned
		for _, role := range s.Roles {
			if role == roleName {
				return nil // Role already assigned
			}
		}
		
		// Add the role
		s.Roles = append(s.Roles, roleName)
		
		// Update the service account in memory
		is.mu.Lock()
		defer is.mu.Unlock()
		is.serviceAccounts[s.ID] = s
		return nil
	default:
		return fmt.Errorf("unsupported subject type for role assignment")
	}
}

// AddUserToGroup adds a user to a group
func (is *IdentityService) AddUserToGroup(user *model.User, groupName string) error {
	// Check if the user is already in the group
	for _, group := range user.Groups {
		if group == groupName {
			return nil // User already in group
		}
	}
	
	// Add the group
	user.Groups = append(user.Groups, groupName)
	
	// Update the user
	return is.UpdateUser(user)
}

// Helper function to check if a string is in a slice
func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// CreateServiceAccount creates a new service account
func (is *IdentityService) CreateServiceAccount(sa *model.ServiceAccount) error {
	is.mu.Lock()
	defer is.mu.Unlock()
	if _, exists := is.serviceAccounts[sa.ID]; exists {
		return fmt.Errorf("service account with ID '%s' already exists", sa.ID)
	}
	is.serviceAccounts[sa.ID] = sa
	return nil
}

// GetServiceAccount retrieves a service account by ID
func (is *IdentityService) GetServiceAccount(saID model.AggregateID) (*model.ServiceAccount, error) {
	is.mu.RLock()
	defer is.mu.RUnlock()
	sa, exists := is.serviceAccounts[saID]
	if !exists {
		return nil, fmt.Errorf("service account with ID '%s' not found", saID)
	}
	return sa, nil
}
