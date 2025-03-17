package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// RoleRepositoryAdapter adapts the RoleMongoDBRepository to implement the RoleRepository interface
type RoleRepositoryAdapter struct {
	repo *RoleMongoDBRepository
}

// NewRoleRepositoryAdapter creates a new RoleRepositoryAdapter
func NewRoleRepositoryAdapter(repo *RoleMongoDBRepository) ports.RoleRepository {
	return &RoleRepositoryAdapter{
		repo: repo,
	}
}

// CreateRole creates a new role
func (a *RoleRepositoryAdapter) CreateRole(role model.Role) error {
	ctx := context.Background()
	// Convert to pointer as the repository expects a pointer
	rolePtr := &role
	_, err := a.repo.Save(ctx, rolePtr)
	return err
}

// GetRole retrieves a role by organization, project, and name
func (a *RoleRepositoryAdapter) GetRole(orgID, projectID, roleName string) (*model.Role, error) {
	ctx := context.Background()
	role, err := a.repo.FindByName(ctx, orgID, projectID, roleName)
	if err != nil {
		if err == generic.ErrNotFound {
			return nil, model.ErrRoleNotFound
		}
		return nil, err
	}
	return role, nil
}

// UpdateRole updates an existing role
func (a *RoleRepositoryAdapter) UpdateRole(role model.Role) error {
	ctx := context.Background()
	// Convert to pointer as the repository expects a pointer
	rolePtr := &role
	return a.repo.Update(ctx, rolePtr)
}

// DeleteRole deletes a role by organization, project, and name
func (a *RoleRepositoryAdapter) DeleteRole(orgID, projectID, roleName string) error {
	ctx := context.Background()
	role, err := a.repo.FindByName(ctx, orgID, projectID, roleName)
	if err != nil {
		if err == generic.ErrNotFound {
			return model.ErrRoleNotFound
		}
		return err
	}
	return a.repo.Delete(ctx, model.AggregateID(role.ID))
}

// ListRoles returns all roles for a specific organization and project
func (a *RoleRepositoryAdapter) ListRoles(orgID, projectID string) ([]model.Role, error) {
	ctx := context.Background()
	rolePtrs, err := a.repo.FindByOrganizationAndProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}

	// Convert from []*model.Role to []model.Role
	roles := make([]model.Role, len(rolePtrs))
	for i, rolePtr := range rolePtrs {
		roles[i] = *rolePtr
	}
	
	return roles, nil
}
