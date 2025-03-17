package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Verification of interface implementations
var _ ports.WriteOnlyRepository[*model.Role] = (*generic.GenericMongoDBWriteRepository[*model.Role, RoleDocument])(nil)
var _ ports.ReadOnlyRepository[*model.Role] = (*generic.GenericMongoDBReadRepository[*model.Role, RoleDocument])(nil)
var _ ports.Repository[*model.Role, model.AggregateID] = (*RoleMongoDBRepository)(nil)

// NewRoleMongoDBWriteRepository creates a write repository for Role
func NewRoleMongoDBWriteRepository(db *mongo.Database, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.Role] {
	converter := NewRoleDocumentConverter(generator)
	return generic.NewGenericMongoDBWriteRepository[*model.Role, RoleDocument](
		db,
		RoleCollection,
		converter,
	)
}

// NewRoleMongoDBReadRepository creates a read repository for Role
func NewRoleMongoDBReadRepository(db *mongo.Database, generator ports.IDGenerator) ports.ReadOnlyRepository[*model.Role] {
	converter := NewRoleDocumentConverter(generator)
	return generic.NewGenericMongoDBReadRepository[*model.Role, RoleDocument](
		db,
		RoleCollection,
		converter,
	)
}

// RoleMongoDBRepository implements the complete Repository interface for Role in MongoDB
type RoleMongoDBRepository struct {
	readRepo  ports.ReadOnlyRepository[*model.Role]
	writeRepo ports.WriteOnlyRepository[*model.Role]
}

// NewRoleMongoDBRepository creates a new instance of the combined repository
func NewRoleMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.Role, model.AggregateID] {
	return &RoleMongoDBRepository{
		readRepo:  NewRoleMongoDBReadRepository(db, generator),
		writeRepo: NewRoleMongoDBWriteRepository(db, generator),
	}
}

// Read methods (ReadOnlyRepository)

// FindByID finds a Role by its ID
func (r *RoleMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Role, error) {
	return r.readRepo.FindByID(ctx, id)
}

// FindAll returns all Roles
func (r *RoleMongoDBRepository) FindAll(ctx context.Context) ([]*model.Role, error) {
	return r.readRepo.FindAll(ctx)
}

// Count returns the total number of Roles
func (r *RoleMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.readRepo.Count(ctx)
}

// Exists checks if a Role with the provided ID exists
func (r *RoleMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.readRepo.Exists(ctx, id)
}

// FindByCriteria searches for Roles applying search criteria and pagination
func (r *RoleMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Role], error) {
	return r.readRepo.FindByCriteria(ctx, criteria)
}

// Write methods (WriteOnlyRepository)

// Save saves a new Role in the database
func (r *RoleMongoDBRepository) Save(ctx context.Context, entity *model.Role) (*model.Role, error) {
	return r.writeRepo.Save(ctx, entity)
}

// Update updates an existing Role in the database
func (r *RoleMongoDBRepository) Update(ctx context.Context, entity *model.Role) error {
	return r.writeRepo.Update(ctx, entity)
}

// Delete deletes a Role by its ID
func (r *RoleMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writeRepo.Delete(ctx, id)
}

// BatchSave saves multiple Roles in the database
func (r *RoleMongoDBRepository) BatchSave(ctx context.Context, entities []*model.Role) ([]*model.Role, error) {

	return r.writeRepo.BatchSave(ctx, entities)
}

// BatchUpdate updates multiple Roles in the database
func (r *RoleMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.Role) error {
	return r.writeRepo.BatchUpdate(ctx, entities)
}

// BatchDelete deletes multiple Roles by their IDs
func (r *RoleMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writeRepo.BatchDelete(ctx, ids)
}

// FindByName finds a role by name within a specific organization and project
func (r *RoleMongoDBRepository) FindByName(ctx context.Context, organization, project, name string) (*model.Role, error) {
	criteria := ports.SearchCriteria{
		Filters: map[string]interface{}{
			"organization": organization,
			"project":      project,
			"name":         name,
		},
		Size: 1,
	}

	result, err := r.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	if result.Size == 0 {
		return nil, generic.ErrNotFound
	}

	return result.Content[0], nil
}

// FindByOrganizationAndProject finds all roles within a specific organization and project
func (r *RoleMongoDBRepository) FindByOrganizationAndProject(ctx context.Context, organization, project string) ([]*model.Role, error) {
	criteria := ports.SearchCriteria{
		Filters: map[string]interface{}{
			"organization": organization,
			"project":      project,
		},
	}

	result, err := r.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	return result.Content, nil
}
