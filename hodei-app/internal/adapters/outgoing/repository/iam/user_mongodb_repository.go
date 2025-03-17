package iam

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Verification of interface implementations
var _ ports.WriteOnlyRepository[*model.UserAuth] = (*generic.GenericMongoDBWriteRepository[*model.UserAuth, UserDocument])(nil)
var _ ports.ReadOnlyRepository[*model.UserAuth] = (*generic.GenericMongoDBReadRepository[*model.UserAuth, UserDocument])(nil)
var _ ports.Repository[*model.UserAuth, model.AggregateID] = (*UserMongoDBRepository)(nil)

// NewUserMongoDBWriteRepository creates a write repository for User
func NewUserMongoDBWriteRepository(db *mongo.Database, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.UserAuth] {
	converter := NewUserDocumentConverter(generator)
	return generic.NewGenericMongoDBWriteRepository[*model.UserAuth, UserDocument](
		db,
		UserCollection,
		converter,
	)
}

// NewUserMongoDBReadRepository creates a read repository for User
func NewUserMongoDBReadRepository(db *mongo.Database, generator ports.IDGenerator) ports.ReadOnlyRepository[*model.UserAuth] {
	converter := NewUserDocumentConverter(generator)
	return generic.NewGenericMongoDBReadRepository[*model.UserAuth, UserDocument](
		db,
		UserCollection,
		converter,
	)
}

// UserMongoDBRepository implements the complete Repository interface for User in MongoDB
type UserMongoDBRepository struct {
	readRepo  ports.ReadOnlyRepository[*model.UserAuth]
	writeRepo ports.WriteOnlyRepository[*model.UserAuth]
}

// NewUserMongoDBRepository creates a new instance of the combined repository
func NewUserMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.UserAuth, model.AggregateID] {
	return &UserMongoDBRepository{
		readRepo:  NewUserMongoDBReadRepository(db, generator),
		writeRepo: NewUserMongoDBWriteRepository(db, generator),
	}
}

// Read methods (ReadOnlyRepository)

// FindByID finds a User by its ID
func (r *UserMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.UserAuth, error) {
	return r.readRepo.FindByID(ctx, id)
}

// FindAll returns all Users
func (r *UserMongoDBRepository) FindAll(ctx context.Context) ([]*model.UserAuth, error) {
	return r.readRepo.FindAll(ctx)
}

// Count returns the total number of Users
func (r *UserMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.readRepo.Count(ctx)
}

// Exists checks if a User with the provided ID exists
func (r *UserMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.readRepo.Exists(ctx, id)
}

// FindByCriteria searches for Users applying search criteria and pagination
func (r *UserMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.UserAuth], error) {
	return r.readRepo.FindByCriteria(ctx, criteria)
}

// Write methods (WriteOnlyRepository)

// Save saves a new User in the database
func (r *UserMongoDBRepository) Save(ctx context.Context, entity *model.UserAuth) (*model.UserAuth, error) {
	return r.writeRepo.Save(ctx, entity)
}

// Update updates an existing User in the database
func (r *UserMongoDBRepository) Update(ctx context.Context, entity *model.UserAuth) error {
	return r.writeRepo.Update(ctx, entity)
}

// Delete deletes a User by its ID
func (r *UserMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writeRepo.Delete(ctx, id)
}

// BatchSave saves multiple Users in the database
func (r *UserMongoDBRepository) BatchSave(ctx context.Context, entities []*model.UserAuth) ([]*model.UserAuth, error) {
	return r.writeRepo.BatchSave(ctx, entities)
}

// BatchUpdate updates multiple Users in the database
func (r *UserMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.UserAuth) error {
	return r.writeRepo.BatchUpdate(ctx, entities)
}

// BatchDelete deletes multiple Users by their IDs
func (r *UserMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writeRepo.BatchDelete(ctx, ids)
}

// FindByUsername finds a user by username
func (r *UserMongoDBRepository) FindByUsername(ctx context.Context, username string) (*model.UserAuth, error) {
	criteria := ports.SearchCriteria{
		Filters: map[string]interface{}{
			"name": username,
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
