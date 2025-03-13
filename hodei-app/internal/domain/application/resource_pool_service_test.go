package usecases_test

import (
	"context"
	usecases "dev.rubentxu.hodei-devops/hodei-app/internal/domain/application"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

type MockResourcePoolRepository struct {
	mock.Mock
	ports.Repository[*model.ResourcePoolDef, model.AggregateID]
}

type MockResourcePoolFactory struct {
	mock.Mock
}

func (m *MockResourcePoolFactory) CreateResourcePool(resourcesDef *model.ResourcePoolDef) (ports.ResourcePool, error) {
	args := m.Called(resourcesDef)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ports.ResourcePool), args.Error(1)
}

func (m *MockResourcePoolFactory) CreateDefaultResourcePool() (ports.ResourcePool, error) {
	args := m.Called()
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(ports.ResourcePool), args.Error(1)
}

func (m *MockResourcePoolRepository) Save(ctx context.Context, entity *model.ResourcePoolDef) (*model.ResourcePoolDef, error) {
	args := m.Called(ctx, entity)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ResourcePoolDef), args.Error(1)
}

func (m *MockResourcePoolRepository) FindByID(ctx context.Context, id model.AggregateID) (*model.ResourcePoolDef, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.ResourcePoolDef), args.Error(1)
}

func (m *MockResourcePoolRepository) Update(ctx context.Context, entity *model.ResourcePoolDef) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockResourcePoolRepository) Delete(ctx context.Context, id model.AggregateID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockResourcePoolRepository) Exists(ctx context.Context, id model.AggregateID) (bool, error) {
	args := m.Called(ctx, id)
	return args.Bool(0), args.Error(1)
}

func (m *MockResourcePoolRepository) FindByCriteria(ctx context.Context, criteria ports.SearchCriteria) (ports.SearchResult[*model.ResourcePoolDef], error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(ports.SearchResult[*model.ResourcePoolDef]), args.Error(1)
}

func createTestResourcePool() *model.ResourcePoolDef {
	return &model.ResourcePoolDef{
		ID: "test-pool-id",
		Metadata: model.Metadata{
			Name:        "Test Pool",
			Description: "Test Pool Description",
			Labels:      []string{"test"},
			Annotations: map[string]string{"env": "test"},
		},
		Spec: model.ResourcePoolSpec{
			PoolID:     "docker-ejemplo",
			Type:       "Docker", // Usar el tipo correcto según la validación
			PoolConfig: map[string]interface{}{},
		},
		Status: model.ResourcePoolStatus{
			State: "PENDING",
		},
	}
}

func TestResourcePoolService(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockResourcePoolRepository{}
	mockFactory := &MockResourcePoolFactory{}

	t.Run("CreateResourcePool", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			pool := createTestResourcePool()
			mockRepo.On("Save", ctx, mock.AnythingOfType("*model.ResourcePoolDef")).Return(pool, nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			result, err := service.CreateResourcePool(ctx, pool)

			require.NoError(t, err)
			assert.Equal(t, pool.ID, result.ID)
			assert.Equal(t, "PENDING", result.Status.State)
			assert.False(t, result.Metadata.CreatedAt.IsZero())
		})

		t.Run("Validation Error", func(t *testing.T) {
			pool := &model.ResourcePoolDef{} // Pool inválido
			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			_, err := service.CreateResourcePool(ctx, pool)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "validation failed")
		})
	})

	t.Run("UpdateResourcePool", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			pool := createTestResourcePool()
			mockRepo.On("FindByID", ctx, pool.ID).Return(pool, nil)
			mockRepo.On("Update", ctx, mock.AnythingOfType("*model.ResourcePoolDef")).Return(nil)

			updates := createTestResourcePool()
			updates.Metadata.Name = "Updated Pool"

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.UpdateResourcePool(ctx, pool.ID, updates)

			require.NoError(t, err)
			mockRepo.AssertExpectations(t)
		})

		t.Run("Not Found", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar llamadas anteriores
			nonexistentID := model.AggregateID("nonexistent")
			mockRepo.On("FindByID", ctx, nonexistentID).Return(nil, fmt.Errorf("not found"))

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.UpdateResourcePool(ctx, nonexistentID, createTestResourcePool())

			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to find pool")
			mockRepo.AssertExpectations(t)
		})
	})

	t.Run("DeleteResourcePool", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar llamadas anteriores
			pool := createTestResourcePool()
			pool.Status.State = "INACTIVE"

			mockRepo.On("FindByID", ctx, pool.ID).Return(pool, nil)
			mockRepo.On("Delete", ctx, pool.ID).Return(nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.DeleteResourcePool(ctx, pool.ID)

			require.NoError(t, err)
			mockRepo.AssertExpectations(t)
		})

		t.Run("Cannot Delete Active", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar llamadas anteriores
			pool := createTestResourcePool()
			pool.Status.State = "ACTIVE"

			mockRepo.On("FindByID", ctx, pool.ID).Return(pool, nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.DeleteResourcePool(ctx, pool.ID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "cannot delete active resource pool")
			mockRepo.AssertExpectations(t)
		})
	})

	t.Run("GetResourcePool", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar llamadas anteriores
			pool := createTestResourcePool()
			mockRepo.On("FindByID", ctx, pool.ID).Return(pool, nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			result, err := service.GetResourcePool(ctx, pool.ID)

			require.NoError(t, err)
			assert.Equal(t, pool.ID, result.ID)
			mockRepo.AssertExpectations(t)
		})

		t.Run("Not Found", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar llamadas anteriores
			nonexistentID := model.AggregateID("nonexistent")
			mockRepo.On("FindByID", ctx, nonexistentID).Return(nil, fmt.Errorf("not found"))

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			_, err := service.GetResourcePool(ctx, nonexistentID)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "not found")
			mockRepo.AssertExpectations(t)
		})
	})

	t.Run("ListResourcePools", func(t *testing.T) {
		t.Run("Success", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil // Limpiar llamadas anteriores
			pools := []*model.ResourcePoolDef{createTestResourcePool(), createTestResourcePool()}
			expectedResult := ports.SearchResult[*model.ResourcePoolDef]{
				Content:       pools,
				TotalElements: 2,
				Page:          1,
				Size:          10,
			}

			// Actualizar el criterio esperado para incluir el SortBy por defecto
			expectedCriteria := ports.SearchCriteria{
				Page:      1,
				Size:      10,
				SortBy:    "metadata.name", // Incluir el valor por defecto que establece el servicio
				SortOrder: "",
			}
			mockRepo.On("FindByCriteria", ctx, expectedCriteria).Return(expectedResult, nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			result, err := service.ListResourcePools(ctx, ports.SearchCriteria{Page: 1, Size: 10})

			require.NoError(t, err)
			assert.Equal(t, len(pools), len(result.Content))
			assert.Equal(t, int64(2), result.TotalElements)
			mockRepo.AssertExpectations(t)
		})

		t.Run("Invalid Criteria", func(t *testing.T) {
			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			_, err := service.ListResourcePools(ctx, ports.SearchCriteria{Page: -1})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid criteria")
		})
	})

	t.Run("Active Pools Management", func(t *testing.T) {
		service := usecases.NewResourcePoolService(mockRepo, mockFactory)

		t.Run("Empty Initial State", func(t *testing.T) {
			pools := service.ListActivePools()
			assert.Empty(t, pools)
		})

		t.Run("Register And Get Active Pool", func(t *testing.T) {
			mockPool := &MockResourcePool{}
			mockPool.On("GetID").Return("test-pool")

			service.(*usecases.ResourcePoolServiceImpl).RegisterActivePool(mockPool)

			pool, exists := service.GetActivePool("test-pool")
			require.True(t, exists)
			assert.NotNil(t, pool)
			assert.Equal(t, "test-pool", (*pool).GetID())
		})

		t.Run("List Active Pools", func(t *testing.T) {
			pools := service.ListActivePools()
			require.Len(t, pools, 1)
			assert.Equal(t, "test-pool", (*pools[0]).GetID())
		})

		t.Run("Unregister Active Pool", func(t *testing.T) {
			err := service.(*usecases.ResourcePoolServiceImpl).UnregisterActivePool("test-pool")
			require.NoError(t, err)

			_, exists := service.GetActivePool("test-pool")
			assert.False(t, exists)
			assert.Empty(t, service.ListActivePools())
		})
	})

	t.Run("CreateResourcePoolInstance", func(t *testing.T) {
		t.Run("Create New Instance", func(t *testing.T) {
			pool := createTestResourcePool()
			pool.Status.State = "ACTIVE"

			mockPool := &MockResourcePool{}
			mockPool.On("GetID").Return(pool.Spec.PoolID)

			mockRepo.On("FindByID", ctx, pool.ID).Return(pool, nil)
			mockFactory.On("CreateResourcePool", pool).Return(mockPool, nil)
			mockRepo.On("Update", ctx, mock.AnythingOfType("*model.ResourcePoolDef")).Return(nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			instance, err := service.CreateResourcePoolInstance(ctx, pool.ID)

			require.NoError(t, err)
			assert.NotNil(t, instance)
			assert.Equal(t, pool.Spec.PoolID, (*instance).GetID())
		})

		t.Run("Return Existing Instance", func(t *testing.T) {
			pool := createTestResourcePool()
			mockPool := &MockResourcePool{}
			mockPool.On("GetID").Return(pool.Spec.PoolID)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			service.(*usecases.ResourcePoolServiceImpl).RegisterActivePool(mockPool)

			instance, err := service.CreateResourcePoolInstance(ctx, pool.ID)

			require.NoError(t, err)
			assert.NotNil(t, instance)
			assert.Equal(t, pool.Spec.PoolID, (*instance).GetID())
			mockRepo.AssertNotCalled(t, "FindByID")
			mockFactory.AssertNotCalled(t, "CreateResourcePool")
		})
	})

	t.Run("CreateAllResourcePools", func(t *testing.T) {
		t.Run("Success - Create Multiple Pools", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil

			pools := []*model.ResourcePoolDef{
				createTestResourcePool(),
				createTestResourcePool(),
			}
			pools[0].Status.State = "ACTIVE"
			pools[1].Status.State = "ACTIVE"
			pools[1].ID = "test-pool-id-2"
			pools[1].Spec.PoolID = "docker-ejemplo-2" // Añadir un PoolID diferente

			searchResult := ports.SearchResult[*model.ResourcePoolDef]{
				Content:       pools,
				TotalElements: 2,
				Page:          1,
				Size:          10,
			}

			expectedCriteria := ports.SearchCriteria{
				Page:      1,
				Size:      100,
				SortBy:    "metadata.name",
				SortOrder: "ASC",
			}

			mockRepo.On("FindByCriteria", ctx, expectedCriteria).Return(searchResult, nil)

			mockPool1 := &MockResourcePool{}
			mockPool1.On("GetID").Return(pools[0].Spec.PoolID)
			mockPool2 := &MockResourcePool{}
			mockPool2.On("GetID").Return(pools[1].Spec.PoolID)

			mockFactory.On("CreateResourcePool", pools[0]).Return(mockPool1, nil)
			mockFactory.On("CreateResourcePool", pools[1]).Return(mockPool2, nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.CreateAllResourcePools(ctx)

			require.NoError(t, err)
			mockRepo.AssertExpectations(t)
			mockFactory.AssertExpectations(t)

			activePools := service.ListActivePools()
			assert.Len(t, activePools, 2)
		})

		t.Run("Partial Success - Some Pools Fail", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil
			mockFactory.ExpectedCalls = nil // Limpiar también las llamadas del factory

			pools := []*model.ResourcePoolDef{
				createTestResourcePool(),
				createTestResourcePool(),
			}
			pools[0].Status.State = "ACTIVE"
			pools[1].Status.State = "ACTIVE"
			pools[1].ID = "test-pool-id-2"
			pools[1].Spec.PoolID = "docker-ejemplo-2"

			searchResult := ports.SearchResult[*model.ResourcePoolDef]{
				Content:       pools,
				TotalElements: 2,
				Page:          1,
				Size:          10,
			}

			expectedCriteria := ports.SearchCriteria{
				Page:      1,
				Size:      100,
				SortBy:    "metadata.name",
				SortOrder: "ASC",
			}

			mockRepo.On("FindByCriteria", ctx, expectedCriteria).Return(searchResult, nil)

			// Configurar el primer pool para éxito
			mockPool1 := &MockResourcePool{}
			mockPool1.On("GetID").Return(pools[0].Spec.PoolID)
			mockFactory.On("CreateResourcePool", pools[0]).Return(mockPool1, nil)

			// Configurar el segundo pool para fallo
			mockFactory.On("CreateResourcePool", pools[1]).Return(nil, fmt.Errorf("creation failed"))

			// Configurar la actualización del estado para el pool que falla
			updatedPool := *pools[1]
			updatedPool.Status.State = "ERROR"
			mockRepo.On("Update", ctx, mock.MatchedBy(func(p *model.ResourcePoolDef) bool {
				return p.ID == pools[1].ID && p.Status.State == "ERROR"
			})).Return(nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.CreateAllResourcePools(ctx)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "se crearon 1 pools con éxito")
			assert.Contains(t, err.Error(), "creation failed")

			activePools := service.ListActivePools()
			assert.Len(t, activePools, 1)
			assert.Equal(t, pools[0].Spec.PoolID, (*activePools[0]).GetID())

			mockRepo.AssertExpectations(t)
			mockFactory.AssertExpectations(t)
		})

		t.Run("No Active Pools", func(t *testing.T) {
			mockRepo.ExpectedCalls = nil

			searchResult := ports.SearchResult[*model.ResourcePoolDef]{
				Content:       []*model.ResourcePoolDef{},
				TotalElements: 0,
				Page:          1,
				Size:          10,
			}

			mockRepo.On("FindByCriteria", ctx, mock.Anything).Return(searchResult, nil)

			service := usecases.NewResourcePoolService(mockRepo, mockFactory)
			err := service.CreateAllResourcePools(ctx)

			require.NoError(t, err)
			assert.Empty(t, service.ListActivePools())
		})
	})
}

// Mock adicional necesario para las pruebas
type MockResourcePool struct {
	mock.Mock
	ports.ResourcePool
}

func (m *MockResourcePool) GetID() string {
	args := m.Called()
	return args.String(0)
}
