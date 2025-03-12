package workerdef_repository

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

const (
	WorkerCollection = "worker_definitions"
)

type WorkerDocument struct {
	ID        string         `bson:"_id"`
	Metadata  WorkerMeta     `bson:"metadata"`
	Spec      WorkerSpecDB   `bson:"spec"`
	Status    WorkerStatusDB `bson:"status"`
	Owner     string         `bson:"owner"`
	TenantID  string         `bson:"tenant_id"`
	CreatedAt time.Time      `bson:"created_at"`
	UpdatedAt time.Time      `bson:"updated_at"`
}

type WorkerMeta struct {
	Name        string            `bson:"name"`
	Description string            `bson:"description,omitempty"`
	Labels      []string          `bson:"labels,omitempty"`
	Annotations map[string]string `bson:"annotations,omitempty"`
	CreatedAt   time.Time         `bson:"created_at"`
	UpdatedAt   time.Time         `bson:"updated_at"`
}

type WorkerSpecDB struct {
	Type        string                 `bson:"instance_type"`
	Image       string                 `bson:"image,omitempty"`
	Env         map[string]string      `bson:"env,omitempty"`
	WorkingDir  string                 `bson:"working_dir,omitempty"`
	Resources   ResourceRequirementsDB `bson:"resources,omitempty"`
	Volumes     []VolumeMountDB        `bson:"volumes,omitempty"`
	Ports       []PortMappingDB        `bson:"ports,omitempty"`
	Labels      map[string]string      `bson:"labels,omitempty"`
	HealthCheck *HealthCheckConfigDB   `bson:"health_check,omitempty"`
	TemplateID  string                 `bson:"template_id,omitempty"`
}

type WorkerStatusDB struct {
	InstanceID string `bson:"instance_id,omitempty"`
	Status     string `bson:"status,omitempty"`
}

type ResourceRequirementsDB struct {
	CPU    float64 `bson:"cpu"`
	Memory string  `bson:"memory"`
}

type VolumeMountDB struct {
	HostPath      string `bson:"host_path"`
	ContainerPath string `bson:"container_path"`
	ReadOnly      bool   `bson:"read_only"`
}

type PortMappingDB struct {
	HostPort      int    `bson:"host_port"`
	ContainerPort int    `bson:"container_port"`
	Protocol      string `bson:"protocol"`
}

type HealthCheckConfigDB struct {
	Type     string `bson:"type"`
	Endpoint string `bson:"endpoint"`
	Interval int64  `bson:"interval"`
	Timeout  int64  `bson:"timeout"`
}

// WorkerDocumentConverter implementa la interfaz DocumentConverter para WorkerDefinition
type WorkerDocumentConverter struct {
	generator ports.IDGenerator
}

func NewWorkerDocumentConverter(generator ports.IDGenerator) generic.DocumentConverter[*model.WorkerDefinition, WorkerDocument] {
	return &WorkerDocumentConverter{
		generator: generator,
	}
}

func (c *WorkerDocumentConverter) GenerateID() model.AggregateID {
	return c.generator.NewID()
}

// ToModel convierte un documento de MongoDB a un modelo de dominio WorkerDefinition
func (c *WorkerDocumentConverter) ToModel(doc WorkerDocument) (*model.WorkerDefinition, error) {
	volumes := make([]model.VolumeMount, len(doc.Spec.Volumes))
	for i, v := range doc.Spec.Volumes {
		volumes[i] = model.VolumeMount{
			HostPath:      v.HostPath,
			ContainerPath: v.ContainerPath,
			ReadOnly:      v.ReadOnly,
		}
	}

	ports := make([]model.PortMapping, len(doc.Spec.Ports))
	for i, p := range doc.Spec.Ports {
		ports[i] = model.PortMapping{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		}
	}

	var healthCheck *model.HealthCheckConfig
	if doc.Spec.HealthCheck != nil {
		healthCheck = &model.HealthCheckConfig{
			Type:     doc.Spec.HealthCheck.Type,
			Endpoint: doc.Spec.HealthCheck.Endpoint,
			Interval: time.Duration(doc.Spec.HealthCheck.Interval) * time.Millisecond,
			Timeout:  time.Duration(doc.Spec.HealthCheck.Timeout) * time.Millisecond,
		}
	}

	return &model.WorkerDefinition{
		ID: model.AggregateID(doc.ID),
		Metadata: model.Metadata{
			Name:        doc.Metadata.Name,
			Description: doc.Metadata.Description,
			Labels:      doc.Metadata.Labels,
			Annotations: doc.Metadata.Annotations,
			CreatedAt:   doc.Metadata.CreatedAt,
			UpdatedAt:   doc.Metadata.UpdatedAt,
		},
		Spec: model.WorkerSpec{
			Type:       model.InstanceType(doc.Spec.Type),
			Image:      doc.Spec.Image,
			Env:        doc.Spec.Env,
			WorkingDir: doc.Spec.WorkingDir,
			Resources: model.ResourceRequirements{
				CPU:    doc.Spec.Resources.CPU,
				Memory: doc.Spec.Resources.Memory,
			},
			Volumes:     volumes,
			Ports:       ports,
			Labels:      doc.Spec.Labels,
			HealthCheck: healthCheck,
			TemplateID:  doc.Spec.TemplateID,
		},
		Status: model.WorkerStatus{
			InstanceID: doc.Status.InstanceID,
			Status:     parseHealthStatus(doc.Status.Status),
		},
	}, nil
}

// ToDocument convierte un modelo de dominio WorkerDefinition a un documento de MongoDB
func (c *WorkerDocumentConverter) ToDocument(entity *model.WorkerDefinition, ctx context.Context) WorkerDocument {
	if entity.ID == "" {
		entity.ID = c.GenerateID()
	}
	now := time.Now().UTC()

	volumes := make([]VolumeMountDB, len(entity.Spec.Volumes))
	for i, v := range entity.Spec.Volumes {
		volumes[i] = VolumeMountDB{
			HostPath:      v.HostPath,
			ContainerPath: v.ContainerPath,
			ReadOnly:      v.ReadOnly,
		}
	}

	ports := make([]PortMappingDB, len(entity.Spec.Ports))
	for i, p := range entity.Spec.Ports {
		ports[i] = PortMappingDB{
			HostPort:      p.HostPort,
			ContainerPort: p.ContainerPort,
			Protocol:      p.Protocol,
		}
	}

	var healthCheck *HealthCheckConfigDB
	if entity.Spec.HealthCheck != nil {
		healthCheck = &HealthCheckConfigDB{
			Type:     entity.Spec.HealthCheck.Type,
			Endpoint: entity.Spec.HealthCheck.Endpoint,
			Interval: int64(entity.Spec.HealthCheck.Interval.Milliseconds()),
			Timeout:  int64(entity.Spec.HealthCheck.Timeout.Milliseconds()),
		}
	}

	return WorkerDocument{
		ID: entity.ID.String(),
		Metadata: WorkerMeta{
			Name:        entity.Metadata.Name,
			Description: entity.Metadata.Description,
			Labels:      entity.Metadata.Labels,
			Annotations: entity.Metadata.Annotations,
			CreatedAt:   entity.Metadata.CreatedAt,
			UpdatedAt:   now,
		},
		Spec: WorkerSpecDB{
			Type:       string(entity.Spec.Type),
			Image:      entity.Spec.Image,
			Env:        entity.Spec.Env,
			WorkingDir: entity.Spec.WorkingDir,
			Resources: ResourceRequirementsDB{
				CPU:    entity.Spec.Resources.CPU,
				Memory: entity.Spec.Resources.Memory,
			},
			Volumes:     volumes,
			Ports:       ports,
			Labels:      entity.Spec.Labels,
			HealthCheck: healthCheck,
			TemplateID:  entity.Spec.TemplateID,
		},
		Status: WorkerStatusDB{
			InstanceID: entity.Status.InstanceID,
			Status:     healthStatusToString(entity.Status.Status),
		},
		CreatedAt: entity.Metadata.CreatedAt,
		UpdatedAt: now,
	}
}

func healthStatusToString(status model.HealthStatus) string {
	switch status {
	case model.HEALTHY:
		return "healthy"
	case model.PENDING:
		return "pending"
	case model.STOPPED:
		return "stopped"
	case model.ERROR:
		return "error"
	default:
		return "unknown"
	}
}

func parseHealthStatus(status string) model.HealthStatus {
	switch status {
	case "healthy":
		return model.HEALTHY
	case "pending":
		return model.PENDING
	case "stopped":
		return model.STOPPED
	case "error":
		return model.ERROR
	default:
		return model.UNKNOWN
	}
}

// BuildFilter construye un filtro BSON a partir de los criterios de búsqueda
func (c *WorkerDocumentConverter) BuildFilter(filters map[string]interface{}) bson.M {
	if filters == nil || len(filters) == 0 {
		return bson.M{}
	}

	filter := bson.M{}

	for key, value := range filters {
		switch key {
		case "name":
			filter["metadata.name"] = value
		case "type":
			filter["spec.instance_type"] = value
		case "image":
			filter["spec.image"] = value
		case "status":
			filter["status.status"] = value
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
		case "templateId":
			filter["spec.template_id"] = value
		}
	}

	return filter
}

// MapSortField mapea el nombre de campo para ordenamiento
func (c *WorkerDocumentConverter) MapSortField(field string) string {
	switch field {
	case "name":
		return "metadata.name"
	case "type":
		return "spec.instance_type"
	case "status":
		return "status.status"
	case "createdAt":
		return "created_at"
	case "updatedAt":
		return "updated_at"
	default:
		return "_id"
	}
}
