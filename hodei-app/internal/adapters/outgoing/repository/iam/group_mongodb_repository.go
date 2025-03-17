package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Verification of interface implementations
var _ ports.WriteOnlyRepository[*model.Group] = (*generic.GenericMongoDBWriteRepository[*model.Group, GroupDocument])(nil)
var _ ports.ReadOnlyRepository[*model.Group] = (*generic.GenericMongoDBReadRepository[*model.Group, GroupDocument])(nil)
var _ ports.Repository[*model.Group, model.AggregateID] = (*GroupMongoDBRepository)(nil)

// NewGroupMongoDBWriteRepository creates a write repository for Group
func NewGroupMongoDBWriteRepository(db *mongo.Database, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.Group] {
	converter := NewGroupDocumentConverter(generator)
	return generic.NewGenericMongoDBWriteRepository[*model.Group, GroupDocument](
		db,
		GroupCollection,
		converter,
	)
}

// NewGroupMongoDBReadRepository creates a read repository for Group
func NewGroupMongoDBReadRepository(db *mongo.Database, generator ports.IDGenerator) ports.ReadOnlyRepository[*model.Group] {
	converter := NewGroupDocumentConverter(generator)
	return generic.NewGenericMongoDBReadRepository[*model.Group, GroupDocument](
		db,
		GroupCollection,
		converter,
	)
}

// GroupMongoDBRepository implements the complete Repository interface for Group in MongoDB
type GroupMongoDBRepository struct {
	readRepo  ports.ReadOnlyRepository[*model.Group]
	writeRepo ports.WriteOnlyRepository[*model.Group]
}

// NewGroupMongoDBRepository creates a new instance of the combined repository
func NewGroupMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.Group, model.AggregateID] {
	return &GroupMongoDBRepository{
		readRepo:  NewGroupMongoDBReadRepository(db, generator),
		writeRepo: NewGroupMongoDBWriteRepository(db, generator),
	}
}

// Read methods (ReadOnlyRepository)

// FindByID finds a Group by its ID
func (r *GroupMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Group, error) {
	return r.readRepo.FindByID(ctx, id)
}

// FindAll returns all Groups
func (r *GroupMongoDBRepository) FindAll(ctx context.Context) ([]*model.Group, error) {
	return r.readRepo.FindAll(ctx)
}

// Count returns the total number of Groups
func (r *GroupMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.readRepo.Count(ctx)
}

// Exists checks if a Group with the provided ID exists
func (r *GroupMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.readRepo.Exists(ctx, id)
}

// FindByCriteria searches for Groups applying search criteria and pagination
func (r *GroupMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Group], error) {
	return r.readRepo.FindByCriteria(ctx, criteria)
}

// Write methods (WriteOnlyRepository)

// Save saves a new Group in the database
func (r *GroupMongoDBRepository) Save(ctx context.Context, entity *model.Group) (*model.Group, error) {
	return r.writeRepo.Save(ctx, entity)
}

// Update updates an existing Group in the database
func (r *GroupMongoDBRepository) Update(ctx context.Context, entity *model.Group) error {
	return r.writeRepo.Update(ctx, entity)
}

// Delete deletes a Group by its ID
func (r *GroupMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writeRepo.Delete(ctx, id)
}

// BatchSave saves multiple Groups in the database
func (r *GroupMongoDBRepository) BatchSave(ctx context.Context, entities []*model.Group) ([]*model.Group, error) {
	return r.writeRepo.BatchSave(ctx, entities)
}

// BatchUpdate updates multiple Groups in the database
func (r *GroupMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.Group) error {
	return r.writeRepo.BatchUpdate(ctx, entities)
}

// BatchDelete deletes multiple Groups by their IDs
func (r *GroupMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writeRepo.BatchDelete(ctx, ids)
}

// FindByName finds a group by name within a specific organization and project
func (r *GroupMongoDBRepository) FindByName(ctx context.Context, organization, project, name string) (*model.Group, error) {
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

// FindByOrganizationAndProject finds all groups within a specific organization and project
func (r *GroupMongoDBRepository) FindByOrganizationAndProject(ctx context.Context, organization, project string) ([]*model.Group, error) {
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
