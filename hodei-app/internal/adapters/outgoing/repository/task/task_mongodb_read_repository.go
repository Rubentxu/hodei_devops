package task_repository

import (
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/mongo"
)

// Verificación de implementación de la interfaz
var _ ports.ReadOnlyRepository[*model.Task] = (*generic.GenericMongoDBReadRepository[*model.Task, TaskDocument])(nil)

// NewTaskMongoDBReadRepository crea un repositorio de lectura para Task
func NewTaskMongoDBReadRepository(db *mongo.Database) ports.ReadOnlyRepository[*model.Task] {
	converter := NewTaskDocumentConverter()
	return generic.NewGenericMongoDBReadRepository[*model.Task, TaskDocument](
		db,
		TaskCollection,
		converter,
	)
}
