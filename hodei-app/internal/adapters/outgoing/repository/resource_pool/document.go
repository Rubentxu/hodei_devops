package rp_repository

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository/generic"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"regexp"
)

const (
	ResourcePoolCollection = "resource_pools"
)

type ResourcePoolDocument struct {
	ID       string                 `bson:"_id"`
	Metadata repository.MetadataDoc `bson:"metadata"`
	Spec     ResourcePoolSpec       `bson:"spec"`
	Status   ResourcePoolStatus     `bson:"status"`
	Owner    string                 `bson:"owner"`
	TenantID string                 `bson:"tenant_id"`
}

type ResourcePoolSpec struct {
	PoolID string                 `bson:"pool_id"`
	Type   string                 `bson:"type"`
	Config map[string]interface{} `bson:"config"`
}

type ResourcePoolStatus struct {
	State string `bson:"state"`
}

// ResourcePoolDocumentConverter implementa la interfaz DocumentConverter para ResourcePoolDef
type ResourcePoolDocumentConverter struct {
	generator ports.IDGenerator
}

func NewResourcePoolDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.ResourcePoolDef, ResourcePoolDocument] {
	return &ResourcePoolDocumentConverter{
		generator: generator,
	}
}

func (c *ResourcePoolDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

// ToModel convierte un documento de MongoDB a un modelo de dominio ResourcePoolDef
func (c *ResourcePoolDocumentConverter) ToModel(doc ResourcePoolDocument) (*model.ResourcePoolDef, error) {
	id := doc.ID

	return &model.ResourcePoolDef{
		ID: model.AggregateID(id),
		Metadata: model.Metadata{
			Name:        doc.Metadata.Name,
			Description: doc.Metadata.Description,
			Labels:      doc.Metadata.Labels,
			Annotations: doc.Metadata.Annotations,
			CreatedAt:   doc.Metadata.CreatedAt,
			UpdatedAt:   doc.Metadata.UpdatedAt,
		},
		Spec: model.ResourcePoolSpec{
			PoolID:     doc.Spec.PoolID,
			Type:       doc.Spec.Type,
			PoolConfig: doc.Spec.Config,
		},
		Status: model.ResourcePoolStatus{
			State: doc.Status.State,
		},
	}, nil
}

// ToDocument convierte un modelo de dominio ResourcePoolDef a un documento de MongoDB
func (c *ResourcePoolDocumentConverter) ToDocument(entity *model.ResourcePoolDef, ctx context.Context) ResourcePoolDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}
	return ResourcePoolDocument{
		ID: entity.ID.String(),
		Metadata: repository.MetadataDoc{
			Name:        entity.Metadata.Name,
			Description: entity.Metadata.Description,
			Labels:      entity.Metadata.Labels,
			Annotations: entity.Metadata.Annotations,
			CreatedAt:   entity.Metadata.CreatedAt,
			UpdatedAt:   entity.Metadata.UpdatedAt,
		},
		Spec: ResourcePoolSpec{
			PoolID: entity.Spec.PoolID,
			Type:   entity.Spec.Type,
			Config: entity.Spec.PoolConfig,
		},
		Status: ResourcePoolStatus{
			State: entity.Status.State,
		},
	}
}

// BuildFilter construye un filtro BSON a partir de los criterios de búsqueda
func (c *ResourcePoolDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	if filters == nil || len(filters) == 0 {
		return bson.M{}
	}

	filter := bson.M{}

	for key, value := range filters {
		switch key {
		case "type":
			filter["spec.type"] = value
		case "name":
			filter["metadata.name"] = value
		case "poolID":
			filter["spec.pool_id"] = value
		case "state":
			filter["status.state"] = value
		case "owner":
			filter["owner"] = value
		case "tenantId":
			filter["tenant_id"] = value
		case "labels":
			if labels, ok := value.([]string); ok && len(labels) > 0 {
				filter["metadata.labels"] = bson.M{"$all": labels}
			}
		case "nameContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.name"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		case "descriptionContains":
			if strValue, ok := value.(string); ok {
				filter["metadata.description"] = bson.M{"$regex": primitive.Regex{
					Pattern: regexp.QuoteMeta(strValue),
					Options: "i",
				}}
			}
		}
	}

	return filter
}

// MapSortField mapea el nombre de campo para ordenamiento
func (c *ResourcePoolDocumentConverter) MapSortField(sortBy string) string {
	switch sortBy {
	case "name":
		return "metadata.name"
	case "type":
		return "spec.type"
	case "poolID":
		return "spec.pool_id"
	case "state":
		return "status.state"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return "_id"
	}
}
