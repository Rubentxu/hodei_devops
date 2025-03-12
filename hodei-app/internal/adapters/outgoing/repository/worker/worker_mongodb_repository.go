package workerdef_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

var _ ports.WriteOnlyRepository[*model.WorkerDefinition] = (*generic.GenericMongoDBWriteRepository[*model.WorkerDefinition, WorkerDocument])(nil)
var _ ports.ReadOnlyRepository[*model.WorkerDefinition] = (*generic.GenericMongoDBReadRepository[*model.WorkerDefinition, WorkerDocument])(nil)
var _ ports.Repository[*model.WorkerDefinition, model.AggregateID] = (*WorkerMongoDBRepository)(nil)

// NewWorkerMongoDBWriteRepository crea un repositorio de escritura para WorkerDefinition
func NewWorkerMongoDBWriteRepository(db *mongo.Database, generator ports.IDGenerator) ports.WriteOnlyRepository[*model.WorkerDefinition] {
	converter := NewWorkerDocumentConverter(generator)
	return generic.NewGenericMongoDBWriteRepository[*model.WorkerDefinition, WorkerDocument](
		db,
		WorkerCollection,
		converter,
	)
}

// NewWorkerMongoDBReadRepository crea un repositorio de lectura para WorkerDefinition
func NewWorkerMongoDBReadRepository(db *mongo.Database, generator ports.IDGenerator) ports.ReadOnlyRepository[*model.WorkerDefinition] {
	converter := NewWorkerDocumentConverter(generator)
	return generic.NewGenericMongoDBReadRepository[*model.WorkerDefinition, WorkerDocument](
		db,
		WorkerCollection,
		converter,
	)
}

// WorkerMongoDBRepository implementa la interfaz Repository completa para WorkerDefinition en MongoDB
type WorkerMongoDBRepository struct {
	readRepo  ports.ReadOnlyRepository[*model.WorkerDefinition]
	writeRepo ports.WriteOnlyRepository[*model.WorkerDefinition]
}

// NewWorkerMongoDBRepository crea una nueva instancia del repositorio combinado
func NewWorkerMongoDBRepository(db *mongo.Database, generator ports.IDGenerator) ports.Repository[*model.WorkerDefinition, model.AggregateID] {
	return &WorkerMongoDBRepository{
		readRepo:  NewWorkerMongoDBReadRepository(db, generator),
		writeRepo: NewWorkerMongoDBWriteRepository(db, generator),
	}
}

// Métodos de lectura (ReadOnlyRepository)

func (r *WorkerMongoDBRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.WorkerDefinition, error) {
	return r.readRepo.FindByID(ctx, id)
}

func (r *WorkerMongoDBRepository) FindAll(ctx context.Context) ([]*model.WorkerDefinition, error) {
	return r.readRepo.FindAll(ctx)
}

func (r *WorkerMongoDBRepository) Count(ctx context.Context) (int64, error) {
	return r.readRepo.Count(ctx)
}

func (r *WorkerMongoDBRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	return r.readRepo.Exists(ctx, id)
}

func (r *WorkerMongoDBRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.WorkerDefinition], error) {
	return r.readRepo.FindByCriteria(ctx, criteria)
}

// Métodos de escritura (WriteOnlyRepository)

func (r *WorkerMongoDBRepository) Save(ctx context.Context, entity *model.WorkerDefinition) (*model.WorkerDefinition, error) {
	return r.writeRepo.Save(ctx, entity)
}

func (r *WorkerMongoDBRepository) Update(ctx context.Context, entity *model.WorkerDefinition) error {
	return r.writeRepo.Update(ctx, entity)
}

func (r *WorkerMongoDBRepository) Delete(ctx context.Context, id model.AggregateID) error {
	return r.writeRepo.Delete(ctx, id)
}

func (r *WorkerMongoDBRepository) BatchSave(ctx context.Context, entities []*model.WorkerDefinition) ([]*model.WorkerDefinition, error) {
	return r.writeRepo.BatchSave(ctx, entities)
}

func (r *WorkerMongoDBRepository) BatchUpdate(ctx context.Context, entities []*model.WorkerDefinition) error {
	return r.writeRepo.BatchUpdate(ctx, entities)
}

func (r *WorkerMongoDBRepository) BatchDelete(ctx context.Context, ids []model.AggregateID) error {
	return r.writeRepo.BatchDelete(ctx, ids)
}
