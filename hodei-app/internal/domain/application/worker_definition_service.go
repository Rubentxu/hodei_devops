package usecases

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"errors"
	"fmt"
	"github.com/go-playground/validator"
	"regexp"
	"time"
)

var (
	ErrWorkerNotFound                               = errors.New("worker definition not found")
	ErrInvalidWorker                                = errors.New("invalid worker definition")
	_                 ports.WorkerDefinitionService = (*WorkerDefinitionServiceImpl)(nil)
	validate                                        = validator.New()
)

func init() {
	// Registro de validadores personalizados
	validate.RegisterValidation("memunit", validateMemoryUnit)
	validate.RegisterValidation("dir", validateDirectory)
}

// validateMemoryUnit valida el formato de memoria (e.j., "1Gi", "512Mi")
func validateMemoryUnit(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	matched, _ := regexp.MatchString(`^\d+(\.\d+)?(Ki|Mi|Gi|Ti)$`, value)
	return matched
}

// validateDirectory valida que la ruta sea válida
func validateDirectory(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	return len(value) > 0 && value[0] == '/'
}

// WorkerDefinitionServiceImpl implementa el puerto del servicio de WorkerDefinition
type WorkerDefinitionServiceImpl struct {
	repo        ports.Repository[*model.WorkerDefinition, model.AggregateID]
	idGenerator ports.IDGenerator
	validate    *validator.Validate
}

func NewWorkerDefinitionService(repo ports.Repository[*model.WorkerDefinition, model.AggregateID], idGen ports.IDGenerator) ports.WorkerDefinitionService {
	return &WorkerDefinitionServiceImpl{
		repo:        repo,
		idGenerator: idGen,
		validate:    validate,
	}
}

// CrearWorkerDefinition crea una nueva definición de worker
func (s *WorkerDefinitionServiceImpl) CreateWorkerDefinition(ctx context.Context, workerDef *model.WorkerDefinition) (*model.WorkerDefinition, error) {

	workerDef.ID = s.idGenerator.NewID()

	if err := s.validateWorkerDefinition(workerDef); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorker, err)
	}

	// Establecer timestamps
	now := time.Now().UTC()
	workerDef.Metadata.CreatedAt = now
	workerDef.Metadata.UpdatedAt = now

	// Estado por defecto
	if workerDef.Status.Status == model.UNKNOWN {
		workerDef.Status.Status = model.PENDING
	}

	return s.repo.Save(ctx, workerDef)
}

// ObtenerWorkerDefinition obtiene una definición de worker por ID
func (s *WorkerDefinitionServiceImpl) GetWorkerDefinition(ctx context.Context, id model.AggregateID) (*model.WorkerDefinition, error) {
	worker, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrWorkerNotFound
	}
	return worker, nil
}

// ActualizarWorkerDefinition actualiza una definición de worker existente
func (s *WorkerDefinitionServiceImpl) UpdateWorkerDefinition(ctx context.Context, updates *model.WorkerDefinition) error {
	if err := s.validateWorkerDefinition(updates); err != nil {
		return err
	}

	existingWorker, err := s.repo.FindByID(ctx, updates.ID)
	if err != nil {
		return ErrWorkerNotFound
	}

	updates.Metadata.CreatedAt = existingWorker.Metadata.CreatedAt
	updates.Metadata.UpdatedAt = time.Now().UTC()

	return s.repo.Update(ctx, updates)
}

// EliminarWorkerDefinition elimina una definición de worker
func (s *WorkerDefinitionServiceImpl) DeleteWorkerDefinition(ctx context.Context, id model.AggregateID) error {
	exists, err := s.repo.Exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrWorkerNotFound
	}

	return s.repo.Delete(ctx, id)
}

// ListarWorkerDefinitions busca definiciones según criterios
func (s *WorkerDefinitionServiceImpl) FindWorkerDefinitions(ctx context.Context, criterio ports.SearchCriteria) (ports.SearchResult[*model.WorkerDefinition], error) {
	return s.repo.FindByCriteria(ctx, criterio)
}

func (s *WorkerDefinitionServiceImpl) FindWorkerDefinitionByName(ctx context.Context, name string) (*model.WorkerDefinition, error) {
	if name == "" {
		return nil, ErrInvalidWorker
	}

	criteria := ports.SearchCriteria{
		Filters: map[string]interface{}{
			"name": name,
		},
		Page: 1,
		Size: 1,
	}

	result, err := s.repo.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, ErrWorkerNotFound
	}

	return result.Content[0], nil
}

// ActualizarEstadoWorker actualiza solo el estado de un worker
func (s *WorkerDefinitionServiceImpl) UpdateWorkerStatus(ctx context.Context, id model.AggregateID, estado model.HealthStatus) error {
	worker, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return ErrWorkerNotFound
	}

	worker.Status.Status = estado
	worker.Metadata.UpdatedAt = time.Now().UTC()

	return s.repo.Update(ctx, worker)
}

// AsociarTemplate asocia un worker con un template
func (s *WorkerDefinitionServiceImpl) AssignTemplate(ctx context.Context, workerID model.AggregateID, templateID string) error {
	worker, err := s.repo.FindByID(ctx, workerID)
	if err != nil {
		return ErrWorkerNotFound
	}

	worker.Spec.TemplateID = templateID
	worker.Metadata.UpdatedAt = time.Now().UTC()

	return s.repo.Update(ctx, worker)
}

func (s *WorkerDefinitionServiceImpl) CreateWorkersBatch(ctx context.Context, workerDefs []*model.WorkerDefinition) ([]*model.WorkerDefinition, error) {
	// Primero asignamos los IDs y timestamps
	now := time.Now().UTC()
	for _, worker := range workerDefs {
		if worker.ID == "" {
			worker.ID = s.idGenerator.NewID()
		}
		worker.Metadata.CreatedAt = now
		worker.Metadata.UpdatedAt = now
		if worker.Status.Status == model.UNKNOWN {
			worker.Status.Status = model.PENDING
		}
	}

	for _, worker := range workerDefs {
		if err := s.validateWorkerDefinition(worker); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidWorker, err)
		}
	}

	return s.repo.BatchSave(ctx, workerDefs)
}

// EliminarWorkerDefinitionsEnLote elimina múltiples definiciones de worker
func (s *WorkerDefinitionServiceImpl) DeleteWorkersBatch(ctx context.Context, ids []model.AggregateID) error {
	return s.repo.BatchDelete(ctx, ids)
}

func (s *WorkerDefinitionServiceImpl) validateWorkerDefinition(worker *model.WorkerDefinition) error {
	if worker == nil {
		return ErrInvalidWorker
	}

	if err := s.validate.Struct(worker); err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			var errorMessages []string
			for _, e := range validationErrors {
				errorMessages = append(errorMessages, formatValidationError(e))
			}
			return fmt.Errorf("validation errors: %v", errorMessages)
		}
		return err
	}

	return nil
}

func formatValidationError(e validator.FieldError) string {
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", e.Field())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", e.Field(), e.Param())
	case "min":
		return fmt.Sprintf("%s must be at least %s", e.Field(), e.Param())
	case "max":
		return fmt.Sprintf("%s must not exceed %s", e.Field(), e.Param())
	case "memunit":
		return fmt.Sprintf("%s must be a valid memory unit (e.g., 1Gi, 512Mi)", e.Field())
	case "dir":
		return fmt.Sprintf("%s must be a valid directory path", e.Field())
	default:
		return fmt.Sprintf("%s failed %s validation", e.Field(), e.Tag())
	}
}
