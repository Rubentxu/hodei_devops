package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
)

// GroupRepositoryAdapter adapts the GroupMongoDBRepository to implement the GroupRepository interface
type GroupRepositoryAdapter struct {
	repo *GroupMongoDBRepository
}

// NewGroupRepositoryAdapter creates a new GroupRepositoryAdapter
func NewGroupRepositoryAdapter(repo *GroupMongoDBRepository) ports.GroupRepository {
	return &GroupRepositoryAdapter{
		repo: repo,
	}
}

// CreateGroup creates a new group
func (a *GroupRepositoryAdapter) CreateGroup(group model.Group) error {
	ctx := context.Background()
	// Convert to pointer as the repository expects a pointer
	groupPtr := &group
	_, err := a.repo.Save(ctx, groupPtr)
	return err
}

// GetGroup retrieves a group by organization, project, and name
func (a *GroupRepositoryAdapter) GetGroup(orgID, projectID, groupName string) (*model.Group, error) {
	ctx := context.Background()
	group, err := a.repo.FindByName(ctx, orgID, projectID, groupName)
	if err != nil {
		if err == generic.ErrNotFound {
			return nil, model.ErrGroupNotFound
		}
		return nil, err
	}
	return group, nil
}

// UpdateGroup updates an existing group
func (a *GroupRepositoryAdapter) UpdateGroup(group model.Group) error {
	ctx := context.Background()
	// Convert to pointer as the repository expects a pointer
	groupPtr := &group
	return a.repo.Update(ctx, groupPtr)
}

// DeleteGroup deletes a group by organization, project, and name
func (a *GroupRepositoryAdapter) DeleteGroup(orgID, projectID, groupName string) error {
	ctx := context.Background()
	group, err := a.repo.FindByName(ctx, orgID, projectID, groupName)
	if err != nil {
		if err == generic.ErrNotFound {
			return model.ErrGroupNotFound
		}
		return err
	}
	return a.repo.Delete(ctx, model.AggregateID(group.ID))
}

// ListGroups returns all groups for a specific organization and project
func (a *GroupRepositoryAdapter) ListGroups(orgID, projectID string) ([]model.Group, error) {
	ctx := context.Background()
	groupPtrs, err := a.repo.FindByOrganizationAndProject(ctx, orgID, projectID)
	if err != nil {
		return nil, err
	}

	// Convert from []*model.Group to []model.Group
	groups := make([]model.Group, len(groupPtrs))
	for i, groupPtr := range groupPtrs {
		groups[i] = *groupPtr
	}

	return groups, nil
}
