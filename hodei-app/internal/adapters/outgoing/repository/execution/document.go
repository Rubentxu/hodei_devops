package task_execution_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"regexp"
	"time"
)

type TaskExecutionDocument struct {
	ID        string                `bson:"_id"`
	Metadata  model.Metadata        `bson:"metadata"`
	Task      model.Task            `bson:"task"`
	Status    model.ExecutionStatus `bson:"status"`
	InputArgs []string              `bson:"input_args"`
	WorkerDef string                `bson:"workerdef_id"`
	CreatedAt time.Time             `bson:"created_at"`
	UpdatedAt time.Time             `bson:"updated_at"`
}

type TaskExecutionDocumentConverter struct {
	generator ports.IDGenerator
}

func NewTaskExecutionDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.TaskExecution, TaskExecutionDocument] {
	return &TaskExecutionDocumentConverter{
		generator: generator,
	}
}

func (c *TaskExecutionDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

func (c *TaskExecutionDocumentConverter) ToModel(doc TaskExecutionDocument) (*model.TaskExecution, error) {
	return &model.TaskExecution{
		ID:        model.AggregateID(doc.ID),
		Metadata:  doc.Metadata,
		Task:      doc.Task,
		Status:    doc.Status,
		InputArgs: doc.InputArgs,
		WorkerDef: model.WorkerDefinition{
			ID: model.AggregateID(doc.WorkerDef),
		},
	}, nil
}

func (c *TaskExecutionDocumentConverter) ToDocument(entity *model.TaskExecution, ctx context.Context) TaskExecutionDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}
	return TaskExecutionDocument{
		ID:        entity.ID.String(),
		Metadata:  entity.Metadata,
		Task:      entity.Task,
		Status:    entity.Status,
		InputArgs: entity.InputArgs,
		WorkerDef: entity.WorkerDef.ID.String(),
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: entity.Metadata.UpdatedAt,
	}
}

func (c *TaskExecutionDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	if filters == nil || len(filters) == 0 {
		return bson.M{}
	}

	filter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			filter["metadata.name"] = value
		case "state":
			filter["status.state"] = value
		case "taskId":
			filter["task.id"] = value
		case "workerdefId":
			filter["workerdef_id"] = value // Asegúrate de que coincida con la estructura del documento
		case "nameContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.name"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		}
	}

	return filter
}

func (c *TaskExecutionDocumentConverter) MapSortField(sortBy string) string {
	switch sortBy {
	case "name":
		return "metadata.name"
	case "state":
		return "status.state"
	case "startTime":
		return "status.start_time"
	case "endTime":
		return "status.end_time"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return "_id"
	}
}
