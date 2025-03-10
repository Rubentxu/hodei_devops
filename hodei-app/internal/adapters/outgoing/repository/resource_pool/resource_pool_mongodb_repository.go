package repository

import (
	"context"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// ResourcePoolMongoDBRepository implementa la interfaz Repository completa para ResourcePoolDef en MongoDB
type ResourcePoolMongoDBRepository struct {
	readRepo  ports.ReadOnlyRepository[*model.ResourcePoolDef, model.AggregateID]
	writeRepo ports.WriteOnlyRepository[*model.ResourcePoolDef, model.AggregateID]
}

// Aseguramos que se implementa la interfaz Repository completa
var _ ports.Repository[*model.ResourcePoolDef, model.AggregateID] = (*ResourcePoolMongoDBRepository)(nil)

// NewResourcePoolMongoDBRepository crea una nueva instancia del repositorio combinado (lectura+escritura)
func NewResourcePoolMongoDBRepository(db *mongo.Database, client *mongo.Client) ports.Repository[*model.ResourcePoolDef, model.AggregateID] {
	return &ResourcePoolMongoDBRepository{
		readRepo:  NewResourcePoolMongoDBReadRepository(db),
		writeRepo: NewResourcePoolMongoDBWriteRepository(db, client),
	}
}

// Métodos de lectura (ReadOnlyRepository)

// FindByID busca un ResourcePoolDef por su ID
func (r *ResourcePoolMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error) {
	return r.readRepo.FindByID(ctx, id)
}

// FindAll retorna todos los ResourcePoolDef
func (r *ResourcePoolMongoDBRepository) FindAll(ctx context.Context) ([]*model.ResourcePoolDef, error) {
	return r.readRepo.FindAll(ctx)
}

// Count devuelve el número total de ResourcePoolDef
func (r *ResourcePoolMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.readRepo.Count(ctx)
}

// Exists verifica si existe un ResourcePoolDef con el ID proporcionado
func (r *ResourcePoolMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.readRepo.Exists(ctx, id)
}

// FindByCriteria busca ResourcePoolDef aplicando criterios de búsqueda y paginación
func (r *ResourcePoolMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.ResourcePoolDef], error) {
	return r.readRepo.FindByCriteria(ctx, criteria)
}

// Métodos de escritura (WriteOnlyRepository)

// Save guarda un nuevo ResourcePoolDef en la base de datos
func (r *ResourcePoolMongoDBRepository) Save(ctx context.Context, entity *model.ResourcePoolDef) error {
	return r.writeRepo.Save(ctx, entity)
}

// Update actualiza un ResourcePoolDef existente en la base de datos
func (r *ResourcePoolMongoDBRepository) Update(ctx context.Context, entity *model.ResourcePoolDef) error {
	return r.writeRepo.Update(ctx, entity)
}

// Delete elimina un ResourcePoolDef por su ID
func (r *ResourcePoolMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writeRepo.Delete(ctx, id)
}

// BatchSave guarda múltiples ResourcePoolDef en la base de datos
func (r *ResourcePoolMongoDBRepository) BatchSave(ctx context.Context, entities []*model.ResourcePoolDef) error {
	return r.writeRepo.BatchSave(ctx, entities)
}

// BatchUpdate actualiza múltiples ResourcePoolDef en la base de datos
func (r *ResourcePoolMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.ResourcePoolDef) error {
	return r.writeRepo.BatchUpdate(ctx, entities)
}

// BatchDelete elimina múltiples ResourcePoolDef por sus IDs
func (r *ResourcePoolMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writeRepo.BatchDelete(ctx, ids)
}

// WithTransaction ejecuta una función dentro de una transacción
func (r *ResourcePoolMongoDBRepository) WithTransaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return r.writeRepo.WithTransaction(ctx, fn)
}