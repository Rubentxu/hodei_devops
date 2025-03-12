package task_execution_repository

import (
    "context"
    "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
    "dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
    "dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
    "go.mongodb.org/mongo-driver/mongo"
)

const (
    TaskExecutionCollection = "task_executions"
)

var _ ports.WriteOnlyRepository[*model.TaskExecution] = (*generic.GenericMongoDBWriteRepository[*model.TaskExecution, TaskExecutionDocument])(nil)
var _ ports.ReadOnlyRepository[*model.TaskExecution] = (*generic.GenericMongoDBReadRepository[*model.TaskExecution, TaskExecutionDocument])(nil)
var _ ports.Repository[*model.TaskExecution, model.AggregateID] = (*TaskExecutionMongoDBRepository)(nil)

type TaskExecutionMongoDBRepository struct {
    readRepo  ports.ReadOnlyRepository[*model.TaskExecution]
    writeRepo ports.WriteOnlyRepository[*model.TaskExecution]
}

func NewTaskExecutionMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.TaskExecution, model.AggregateID] {
    return &TaskExecutionMongoDBRepository{
        readRepo:  NewTaskExecutionMongoDBReadRepository(db, generator),
        writeRepo: NewTaskExecutionMongoDBWriteRepository(db, generator),
    }
}

func NewTaskExecutionMongoDBWriteRepository(db *mongo.Database, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.TaskExecution] {
    converter := NewTaskExecutionDocumentConverter(generator)
    return generic.NewGenericMongoDBWriteRepository[*model.TaskExecution, TaskExecutionDocument](
        db,
        TaskExecutionCollection,
        converter,
    )
}

func NewTaskExecutionMongoDBReadRepository(db *mongo.Database, generator ports.IDGenerator) ports.ReadOnlyRepository[*model.TaskExecution] {
    converter := NewTaskExecutionDocumentConverter(generator)
    return generic.NewGenericMongoDBReadRepository[*model.TaskExecution, TaskExecutionDocument](
        db,
        TaskExecutionCollection,
        converter,
    )
}

// Implementación de métodos de lectura
func (r *TaskExecutionMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.TaskExecution, error) {
    return r.readRepo.FindByID(ctx, id)
}

func (r *TaskExecutionMongoDBRepository) FindAll(ctx context.Context) ([]*model.TaskExecution, error) {
    return r.readRepo.FindAll(ctx)
}

func (r *TaskExecutionMongoDBRepository) Count(ctx context.Context) (int64, error) {
    return r.readRepo.Count(ctx)
}

func (r *TaskExecutionMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
    return r.readRepo.Exists(ctx, id)
}

func (r *TaskExecutionMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.TaskExecution], error) {
    return r.readRepo.FindByCriteria(ctx, criteria)
}

// Implementación de métodos de escritura
func (r *TaskExecutionMongoDBRepository) Save(ctx context.Context, entity *model.TaskExecution) (*model.TaskExecution, error) {
    return r.writeRepo.Save(ctx, entity)
}

func (r *TaskExecutionMongoDBRepository) Update(ctx context.Context, entity *model.TaskExecution) error {
    return r.writeRepo.Update(ctx, entity)
}

func (r *TaskExecutionMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
    return r.writeRepo.Delete(ctx, id)
}

func (r *TaskExecutionMongoDBRepository) BatchSave(ctx context.Context, entities []*model.TaskExecution) ([]*model.TaskExecution, error) {
    return r.writeRepo.BatchSave(ctx, entities)
}

func (r *TaskExecutionMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.TaskExecution) error {
    return r.writeRepo.BatchUpdate(ctx, entities)
}

func (r *TaskExecutionMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
    return r.writeRepo.BatchDelete(ctx, ids)
}