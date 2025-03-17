package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// UserRepositoryAdapter adapts the UserMongoDBRepository to implement the UserRepository interface
type UserRepositoryAdapter struct {
	repo *UserMongoDBRepository
}

// NewUserRepositoryAdapter creates a new UserRepositoryAdapter
func NewUserRepositoryAdapter(repo *UserMongoDBRepository) ports.UserRepository {
	return &UserRepositoryAdapter{
		repo: repo,
	}
}

// CreateUser creates a new user
func (a *UserRepositoryAdapter) CreateUser(user *model.UserAuth) error {
	ctx := context.Background()
	_, err := a.repo.Save(ctx, user)
	return err
}

// GetUserByID retrieves a user by ID
func (a *UserRepositoryAdapter) GetUserByID(id string) (*model.UserAuth, error) {
	ctx := context.Background()
	return a.repo.FindByID(ctx, model.AggregateID(id))
}

// GetUserByUsername retrieves a user by username
func (a *UserRepositoryAdapter) GetUserByUsername(username string) (*model.UserAuth, error) {
	ctx := context.Background()
	user, err := a.repo.FindByUsername(ctx, username)
	if err != nil {
		if err == generic.ErrNotFound {
			return nil, model.ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

// UpdateUser updates an existing user
func (a *UserRepositoryAdapter) UpdateUser(user *model.UserAuth) error {
	ctx := context.Background()
	return a.repo.Update(ctx, user)
}

// DeleteUser deletes a user
func (a *UserRepositoryAdapter) DeleteUser(id string) error {
	ctx := context.Background()
	return a.repo.Delete(ctx, model.AggregateID(id))
}

// ListUsers returns all users
func (a *UserRepositoryAdapter) ListUsers() ([]*model.UserAuth, error) {
	ctx := context.Background()
	return a.repo.FindAll(ctx)
}
