package usecases

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"github.com/go-playground/validator"
	"log"
	"strings"
	"sync"
	"time"
)

var _ ports.ResourcePoolService = (*ResourcePoolServiceImpl)(nil)

type ResourcePoolServiceImpl struct {
	repo                ports.Repository[*model.ResourcePoolDef, model.AggregateID]
	validator           *validator.Validate
	resourcePoolFactory ports.ResourcePoolFactory
	activePools         map[string]ports.ResourcePool
	mu                  sync.RWMutex
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

	return &ResourcePoolServiceImpl{
		repo:                repo,
		resourcePoolFactory: factory,
		validator:           validate,
		activePools:         make(map[string]ports.ResourcePool),
	}
}

// RegisterActivePool registra un ResourcePool como activo
func (s *ResourcePoolServiceImpl) RegisterActivePool(pool ports.ResourcePool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activePools[pool.GetID()] = pool
	log.Printf("ResourcePool %s registrado como activo", pool.GetID())
}

// UnregisterActivePool elimina un ResourcePool de la lista de activos
func (s *ResourcePoolServiceImpl) UnregisterActivePool(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.activePools[id]; !exists {
		return fmt.Errorf("pool de recursos %s no encontrado", id)
	}
	delete(s.activePools, id)
	log.Printf("ResourcePool %s eliminado del registro", id)
	return nil
}

// GetActivePool obtiene un pool activo por su ID
func (s *ResourcePoolServiceImpl) GetActivePool(id string) (ports.ResourcePool, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pool, exists := s.activePools[id]
	return pool, exists
}

// ListActivePools lista todos los pools activos
func (s *ResourcePoolServiceImpl) ListActivePools() []ports.ResourcePool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pools := make([]ports.ResourcePool, 0, len(s.activePools))
	for _, pool := range s.activePools {
		pools = append(pools, pool)
	}
	return pools
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

func (s *ResourcePoolServiceImpl) CreateAllResourcePools(ctx context.Context) error {
	// Buscar todas las definiciones de pools en estado ACTIVE
	criteria := ports.SearchCriteria{
		Page:      1,
		Size:      100, // Ajustar según necesidades
		SortBy:    "metadata.name",
		SortOrder: "ASC",
	}

	result, err := s.repo.FindByCriteria(ctx, criteria)
	if err != nil {
		return fmt.Errorf("error al buscar definiciones de pools: %w", err)
	}

	var errors []string
	successCount := 0

	// Intentar crear una instancia para cada definición activa
	for _, poolDef := range result.Content {
		if poolDef.Status.State != "ACTIVE" {
			continue
		}

		if _, exists := s.GetActivePool(string(poolDef.ID)); exists {
			successCount++
			continue
		}

		pool, err := s.resourcePoolFactory.CreateResourcePool(poolDef)
		if err != nil {
			errors = append(errors, fmt.Sprintf("error al crear pool %s: %v", poolDef.ID, err))
			// Actualizar estado a ERROR
			poolDef.Status.State = "ERROR"
			poolDef.Metadata.UpdatedAt = time.Now().UTC()
			if updateErr := s.repo.Update(ctx, poolDef); updateErr != nil {
				errors = append(errors, fmt.Sprintf("error al actualizar estado del pool %s: %v", poolDef.ID, updateErr))
			}
			continue
		}

		// Verificar que el ID coincide
		if pool.GetID() != poolDef.Spec.PoolID {
			errors = append(errors, fmt.Sprintf("ID del pool no coincide para %s: esperado %s, obtenido %s",
				poolDef.ID, poolDef.Spec.PoolID, pool.GetID()))
			continue
		}

		s.RegisterActivePool(pool)
		successCount++
	}

	// Si hubo errores, retornar un error consolidado
	if len(errors) > 0 {
		return fmt.Errorf("se crearon %d pools con éxito, pero ocurrieron %d errores:\n%s",
			successCount, len(errors), strings.Join(errors, "\n"))
	}

	return nil
}

// CreateResourcePoolInstance crea una instancia de ResourcePool a partir de su definición
func (s *ResourcePoolServiceImpl) CreateResourcePoolInstance(ctx context.Context, id model.AggregateID) (ports.ResourcePool, error) {
	// Primero verifica si ya existe un pool activo
	if pool, exists := s.GetActivePool(string(id)); exists {
		return pool, nil
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

	pool, err := s.resourcePoolFactory.CreateResourcePool(poolDef)
	if err != nil {
		poolDef.Status.State = "ERROR"
		if updateErr := s.repo.Update(ctx, poolDef); updateErr != nil {
			return nil, fmt.Errorf("error al crear instancia: %v y al actualizar estado: %v", err, updateErr)
		}
		return nil, fmt.Errorf("error al crear instancia del pool: %w", err)
	}

	s.RegisterActivePool(pool)

	// Verificar que el tipo de pool creado coincide con la definición
	if pool.GetID() != poolDef.Spec.PoolID {
		return nil, fmt.Errorf("pool ID mismatch: expected %s, got %s", poolDef.Spec.PoolID, pool.GetID())
	}

	// Actualizar el estado a ACTIVE si todo fue exitoso
	poolDef.Status.State = "ACTIVE"
	poolDef.Metadata.UpdatedAt = time.Now().UTC()

	if err := s.repo.Update(ctx, poolDef); err != nil {
		s.UnregisterActivePool(pool.GetID())
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
