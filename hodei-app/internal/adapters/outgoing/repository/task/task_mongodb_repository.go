package task_repository

import (
	"context"

	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Aseguramos que la implementación cumpla con la interfaz Repository
var _ ports.Repository[*model.Task, model.AggregateID] = (*TaskMongoDBRepository)(nil)

// TaskMongoDBRepository implementa la interfaz Repository para Task combinando los repositorios de lectura y escritura
type TaskMongoDBRepository struct {
	reader ports.ReadOnlyRepository[*model.Task, model.AggregateID]
	writer ports.WriteOnlyRepository[*model.Task, model.AggregateID]
}

// NewTaskMongoDBRepository crea una nueva instancia del repositorio combinado para MongoDB
func NewTaskMongoDBRepository(db *mongo.Database, client *mongo.Client) ports.Repository[*model.Task, model.AggregateID] {
	return &TaskMongoDBRepository{
		reader: NewTaskMongoDBReadRepository(db),
		writer: NewTaskMongoDBWriteRepository(db, client),
	}
}

// Implementación de los métodos de ReadOnlyRepository

// FindByID recupera una Task por su ID
func (r *TaskMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	return r.reader.FindByID(ctx, id)
}

// FindAll recupera todas las Tasks
func (r *TaskMongoDBRepository) FindAll(ctx context.Context) ([]*model.Task, error) {
	return r.reader.FindAll(ctx)
}

// Count devuelve el número total de Tasks
func (r *TaskMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.reader.Count(ctx)
}

// Exists verifica si existe una Task con el ID proporcionado
func (r *TaskMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.reader.Exists(ctx, id)
}

// FindByCriteria busca Tasks aplicando criterios de búsqueda y paginación
func (r *TaskMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	return r.reader.FindByCriteria(ctx, criteria)
}

// Implementación de los métodos de WriteOnlyRepository

// Save guarda una nueva Task en la base de datos
func (r *TaskMongoDBRepository) Save(ctx context.Context, entity *model.Task) error {
	return r.writer.Save(ctx, entity)
}

// Update actualiza una Task existente en la base de datos
func (r *TaskMongoDBRepository) Update(ctx context.Context, entity *model.Task) error {
	return r.writer.Update(ctx, entity)
}

// Delete elimina una Task por su ID
func (r *TaskMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writer.Delete(ctx, id)
}

// BatchSave guarda múltiples Tasks en la base de datos
func (r *TaskMongoDBRepository) BatchSave(ctx context.Context, entities []*model.Task) error {
	return r.writer.BatchSave(ctx, entities)
}

// BatchUpdate actualiza múltiples Tasks en la base de datos
func (r *TaskMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.Task) error {
	return r.writer.BatchUpdate(ctx, entities)
}

// BatchDelete elimina múltiples Tasks por sus IDs
func (r *TaskMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writer.BatchDelete(ctx, ids)
}
