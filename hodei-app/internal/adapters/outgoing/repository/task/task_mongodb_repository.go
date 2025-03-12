package task_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Verificación de implementación de interfaces
var _ ports.WriteOnlyRepository[*model.Task] = (*generic.GenericMongoDBWriteRepository[*model.Task, TaskDocument])(nil)
var _ ports.ReadOnlyRepository[*model.Task] = (*generic.GenericMongoDBReadRepository[*model.Task, TaskDocument])(nil)
var _ ports.Repository[*model.Task, model.AggregateID] = (*TaskMongoDBRepository)(nil)

// NewTaskMongoDBWriteRepository crea un repositorio de escritura para Task
func NewTaskMongoDBWriteRepository(db *mongo.Database, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.Task] {
	converter := NewTaskDocumentConverter(generator)
	return generic.NewGenericMongoDBWriteRepository[*model.Task, TaskDocument](
		db,
		TaskCollection,
		converter,
	)
}

// NewTaskMongoDBReadRepository crea un repositorio de lectura para Task
func NewTaskMongoDBReadRepository(db *mongo.Database, generator ports.IDGenerator) ports.ReadOnlyRepository[*model.Task] {
	converter := NewTaskDocumentConverter(generator)
	return generic.NewGenericMongoDBReadRepository[*model.Task, TaskDocument](
		db,
		TaskCollection,
		converter,
	)
}

// TaskMongoDBRepository implementa la interfaz Repository completa para Task en MongoDB
type TaskMongoDBRepository struct {
	readRepo  ports.ReadOnlyRepository[*model.Task]
	writeRepo ports.WriteOnlyRepository[*model.Task]
}

// NewTaskMongoDBRepository crea una nueva instancia del repositorio combinado
func NewTaskMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.Task, model.AggregateID] {
	return &TaskMongoDBRepository{
		readRepo:  NewTaskMongoDBReadRepository(db, generator),
		writeRepo: NewTaskMongoDBWriteRepository(db, generator),
	}
}

// Métodos de lectura (ReadOnlyRepository)

// FindByID busca un Task por su ID
func (r *TaskMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.Task, error) {
	return r.readRepo.FindByID(ctx, id)
}

// FindAll retorna todos los Task
func (r *TaskMongoDBRepository) FindAll(ctx context.Context) ([]*model.Task, error) {
	return r.readRepo.FindAll(ctx)
}

// Count devuelve el número total de Task
func (r *TaskMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.readRepo.Count(ctx)
}

// Exists verifica si existe un Task con el ID proporcionado
func (r *TaskMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.readRepo.Exists(ctx, id)
}

// FindByCriteria busca Task aplicando criterios de búsqueda y paginación
func (r *TaskMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.Task], error) {
	return r.readRepo.FindByCriteria(ctx, criteria)
}

// Métodos de escritura (WriteOnlyRepository)

// Save guarda un nuevo Task en la base de datos
func (r *TaskMongoDBRepository) Save(ctx context.Context, entity *model.Task) (*model.Task, error) {
	return r.writeRepo.Save(ctx, entity)
}

// Update actualiza un Task existente en la base de datos
func (r *TaskMongoDBRepository) Update(ctx context.Context, entity *model.Task) error {
	return r.writeRepo.Update(ctx, entity)
}

// Delete elimina un Task por su ID
func (r *TaskMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writeRepo.Delete(ctx, id)
}

// BatchSave guarda múltiples Task en la base de datos
func (r *TaskMongoDBRepository) BatchSave(ctx context.Context, entities []*model.Task) ([]*model.Task, error) {
	return r.writeRepo.BatchSave(ctx, entities)
}

// BatchUpdate actualiza múltiples Task en la base de datos
func (r *TaskMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.Task) error {
	return r.writeRepo.BatchUpdate(ctx, entities)
}

// BatchDelete elimina múltiples Task por sus IDs
func (r *TaskMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writeRepo.BatchDelete(ctx, ids)
}
