package workerdef_repository

import (
	"context"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Aseguramos que la implementación cumpla con la interfaz Repository
var _ ports.Repository[*model.WorkerDefinition, model.AggregateID] = (*WorkerMongoDBRepository)(nil)

// WorkerMongoDBRepository implementa la interfaz Repository para WorkerDefinition combinando los repositorios de lectura y escritura
type WorkerMongoDBRepository struct {
	reader ports.ReadOnlyRepository[*model.WorkerDefinition, model.AggregateID]
	writer *WorkerMongoDBWriteRepository
}

// NewWorkerMongoDBRepository crea una nueva instancia del repositorio combinado para MongoDB
func NewWorkerMongoDBRepository(db *mongo.Database, client *mongo.Client) *WorkerMongoDBRepository {
	return &WorkerMongoDBRepository{
		reader: NewWorkerMongoDBReadRepository(db),
		writer: NewWorkerMongoDBWriteRepository(db, client).(*WorkerMongoDBWriteRepository),
	}
}

// Implementación de los métodos de ReadOnlyRepository

// FindByID recupera un WorkerDefinition por su ID
func (r *WorkerMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.WorkerDefinition, error) {
	return r.reader.FindByID(ctx, id)
}

// FindAll recupera todos los WorkerDefinition
func (r *WorkerMongoDBRepository) FindAll(ctx context.Context) ([]*model.WorkerDefinition, error) {
	return r.reader.FindAll(ctx)
}

// Count devuelve el número total de WorkerDefinition
func (r *WorkerMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.reader.Count(ctx)
}

// Exists verifica si existe un WorkerDefinition con el ID proporcionado
func (r *WorkerMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.reader.Exists(ctx, id)
}

// FindByCriteria busca WorkerDefinition aplicando criterios de búsqueda y paginación
func (r *WorkerMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.WorkerDefinition], error) {
	return r.reader.FindByCriteria(ctx, criteria)
}

// Implementación de los métodos de WriteOnlyRepository

// Save guarda un nuevo WorkerDefinition en la base de datos
func (r *WorkerMongoDBRepository) Save(ctx context.Context, entity *model.WorkerDefinition) error {
	return r.writer.Save(ctx, entity)
}

// Update actualiza un WorkerDefinition existente en la base de datos
func (r *WorkerMongoDBRepository) Update(ctx context.Context, entity *model.WorkerDefinition) error {
	return r.writer.Update(ctx, entity)
}

// Delete elimina un WorkerDefinition por su ID
func (r *WorkerMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writer.Delete(ctx, id)
}

// BatchSave guarda múltiples WorkerDefinition en la base de datos
func (r *WorkerMongoDBRepository) BatchSave(ctx context.Context, entities []*model.WorkerDefinition) error {
	return r.writer.BatchSave(ctx, entities)
}

// BatchUpdate actualiza múltiples WorkerDefinition en la base de datos
func (r *WorkerMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.WorkerDefinition) error {
	return r.writer.BatchUpdate(ctx, entities)
}

// BatchDelete elimina múltiples WorkerDefinition por sus IDs
func (r *WorkerMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writer.BatchDelete(ctx, ids)
}
