package usecases

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"github.com/go-playground/validator"
	"time"
)

type ResourcePoolServiceImpl struct {
	repo                ports.Repository[*model.ResourcePoolDef, model.AggregateID]
	validator           *validator.Validate
	resourcePoolFactory ports.ResourcePoolFactory
}

func NewResourcePoolService(
	repo ports.Repository[*model.ResourcePoolDef, model.AggregateID],
	factory ports.ResourcePoolFactory,
) ports.ResourcePoolService {
	if repo == nil {
		panic("repository cannot be nil")
	}
	if factory == nil {
		panic("resource pool factory cannot be nil")
	}
	validate := validator.New()
	validate.RegisterValidation("pooltype", model.ValidatePoolType)
	return &ResourcePoolServiceImpl{repo: repo, resourcePoolFactory: factory}
}

func (s *ResourcePoolServiceImpl) CreateResourcePool(ctx context.Context, resourceDef *model.ResourcePoolDef) (*model.ResourcePoolDef, error) {
	if err := s.validator.Struct(resourceDef); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	now := time.Now().UTC()
	if resourceDef.Metadata.CreatedAt.IsZero() {
		resourceDef.Metadata.CreatedAt = now
	}
	resourceDef.Metadata.UpdatedAt = now

	if resourceDef.Status.State == "" {
		resourceDef.Status.State = "PENDING"
	}
	return s.repo.Save(ctx, resourceDef)
}

// CreateResourcePoolInstance crea una instancia de ResourcePool a partir de su definición
func (s *ResourcePoolServiceImpl) CreateResourcePoolInstance(ctx context.Context, id model.AggregateID) (ports.ResourcePool, error) {
	if id == "" {
		return nil, fmt.Errorf("id cannot be empty")
	}

	// Recuperar la definición del pool
	poolDef, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to find pool definition: %w", err)
	}

	// Validar el estado del pool
	if poolDef.Status.State != "ACTIVE" {
		return nil, fmt.Errorf("cannot create instance for pool in state: %s", poolDef.Status.State)
	}

	// Validar la definición completa
	if err := s.validator.Struct(poolDef); err != nil {
		return nil, fmt.Errorf("invalid pool definition: %w", err)
	}

	// Crear la instancia del pool usando el factory
	pool, err := s.resourcePoolFactory.CreateResourcePool(poolDef.Spec.ExtendedSpec, nil)
	if err != nil {
		// Actualizar el estado a ERROR si falla la creación
		poolDef.Status.State = "ERROR"
		if updateErr := s.repo.Update(ctx, poolDef); updateErr != nil {
			return nil, fmt.Errorf("failed to create pool instance: %v and failed to update status: %v", err, updateErr)
		}
		return nil, fmt.Errorf("failed to create pool instance: %w", err)
	}

	// Verificar que el tipo de pool creado coincide con la definición
	if pool.GetID() != poolDef.Spec.PoolID {
		return nil, fmt.Errorf("pool ID mismatch: expected %s, got %s", poolDef.Spec.PoolID, pool.GetID())
	}

	// Actualizar el estado a ACTIVE si todo fue exitoso
	poolDef.Status.State = "ACTIVE"
	poolDef.Metadata.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, poolDef); err != nil {
		return nil, fmt.Errorf("pool instance created but failed to update status: %w", err)
	}

	return pool, nil
}

func (s *ResourcePoolServiceImpl) UpdateResourcePool(ctx context.Context, id model.AggregateID, updates *model.ResourcePoolDef) error {
	if err := s.validator.Var(id, "required"); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}

	if err := s.validator.Struct(updates); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find pool: %w", err)
	}

	updates.ID = current.ID
	updates.Metadata.CreatedAt = current.Metadata.CreatedAt
	updates.Metadata.UpdatedAt = time.Now().UTC()

	return s.repo.Update(ctx, updates)
}

func (s *ResourcePoolServiceImpl) DeleteResourcePool(ctx context.Context, id model.AggregateID) error {
	if err := s.validator.Var(id, "required"); err != nil {
		return fmt.Errorf("invalid id: %w", err)
	}

	pool, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to find pool: %w", err)
	}

	if pool.Status.State == "ACTIVE" {
		return fmt.Errorf("cannot delete active resource pool")
	}

	return s.repo.Delete(ctx, id)
}

func (s *ResourcePoolServiceImpl) GetResourcePool(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error) {
	if err := s.validator.Var(id, "required"); err != nil {
		return nil, fmt.Errorf("invalid id: %w", err)
	}

	return s.repo.FindByID(ctx, id)
}

func (s *ResourcePoolServiceImpl) ListResourcePools(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.ResourcePoolDef], error) {
	if err := s.validator.Struct(criteria); err != nil {
		return ports.SearchResult[*model.ResourcePoolDef]{}, fmt.Errorf("invalid criteria: %w", err)
	}

	if criteria.Page < 1 {
		criteria.Page = 1
	}
	if criteria.Size < 1 {
		criteria.Size = 10
	}
	if criteria.Size > 100 {
		criteria.Size = 100
	}

	if criteria.SortBy == "" {
		criteria.SortBy = "metadata.name"
	}

	return s.repo.FindByCriteria(ctx, criteria)
}
