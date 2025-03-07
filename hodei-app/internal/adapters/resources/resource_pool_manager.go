package resources

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"dev.rubentxu.devops-platform/orchestrator/internal/ports"
)

// ResourcePoolManager gestiona la persistencia, recuperación y acceso a las
// instancias activas de ResourcePool en el sistema
// Usamos punteros para los ResourcePools para mejorar la eficiencia y consistencia
type ResourcePoolManager struct {
	templateStore       ports.Store[ports.WorkerTemplate]
	configStore         ports.Store[map[string]interface{}]
	resourcePoolFactory ports.ResourcePoolFactory
	activePools         map[string]*ports.ResourcePool
	mu                  sync.RWMutex // Mutex para operaciones concurrentes sobre activePools
}

// NewResourcePoolManager crea un nuevo gestor de ResourcePools
func NewResourcePoolManager(
	configStore ports.Store[map[string]interface{}],
	templateStore ports.Store[ports.WorkerTemplate],
	resourcePoolFactory ports.ResourcePoolFactory,
) (*ResourcePoolManager, error) {
	return &ResourcePoolManager{
		templateStore:       templateStore,
		configStore:         configStore,
		resourcePoolFactory: resourcePoolFactory,
		activePools:         make(map[string]*ports.ResourcePool),
	}, nil
}

// SaveConfig guarda una configuración de ResourcePool en la base de datos
func (m *ResourcePoolManager) SaveConfig(config ports.ResourcePoolConfig) error {
	// Convertir la configuración a un mapa genérico para almacenamiento
	var configMap map[string]interface{}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Añadir campos necesarios
	configMap["type"] = config.GetType()
	configMap["name"] = config.GetName()
	configMap["description"] = config.GetDescription()

	// Guardar en el store
	if err := m.configStore.Put(config.GetName(), configMap); err != nil {
		return fmt.Errorf("error saving config: %w", err)
	}

	return nil
}

// GetConfig obtiene una configuración de ResourcePool por nombre
func (m *ResourcePoolManager) GetConfig(name string) (map[string]interface{}, error) {
	config, err := m.configStore.Get(name)
	if err != nil {
		return nil, fmt.Errorf("error getting config: %w", err)
	}
	return config, nil
}

// DeleteConfig elimina una configuración de ResourcePool
func (m *ResourcePoolManager) DeleteConfig(name string) error {
	// Verificar si el pool está activo
	if _, exists := m.GetActivePool(name); exists {
		return fmt.Errorf("cannot delete config for active pool: %s. deactivate it first", name)
	}

	if err := m.configStore.Delete(name); err != nil {
		return fmt.Errorf("error deleting config: %w", err)
	}
	return nil
}

// ListConfigs lista todas las configuraciones de ResourcePool
func (m *ResourcePoolManager) ListConfigs() ([]map[string]interface{}, error) {
	return m.configStore.List()
}

// CreateResourcePool crea una instancia de ResourcePool a partir de una configuración
func (m *ResourcePoolManager) CreateResourcePool(configName string) error {
	// Verificar si ya existe un pool activo con ese nombre
	if _, exists := m.GetActivePool(configName); exists {
		return nil // Devolvemos el existente
	}

	// Obtener la configuración
	config, err := m.GetConfig(configName)
	if err != nil {
		return fmt.Errorf("error getting config: %w", err)
	}

	// Crear la instancia de ResourcePool
	pool, err := m.resourcePoolFactory.CreateResourcePool(config, m.templateStore)
	if err != nil {
		return fmt.Errorf("error creating resource pool: %w", err)
	}

	// Registrar el pool como activo
	m.RegisterActivePool(&pool)

	return nil
}

// CreateAllResourcePools crea instancias de ResourcePool para todas las configuraciones
func (m *ResourcePoolManager) CreateAllResourcePools() error {
	configs, err := m.ListConfigs()
	if err != nil {
		return fmt.Errorf("error listing configs: %w", err)
	}

	for _, config := range configs {
		name, ok := config["name"].(string)
		if !ok {
			log.Printf("Warning: Config without name, skipping: %v", config)
			continue
		}

		pool, err := m.resourcePoolFactory.CreateResourcePool(config, m.templateStore)
		if err != nil {
			log.Printf("Error creating resource pool %s: %v", name, err)
			continue
		}

		// Registrar el pool como activo
		m.RegisterActivePool(&pool)
		log.Printf("Created resource pool: %s", name)
	}

	return nil
}

// ---------- Métodos para gestionar pools activos ----------

// RegisterActivePool registra un ResourcePool como activo
func (m *ResourcePoolManager) RegisterActivePool(pool *ports.ResourcePool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activePools[(*pool).GetID()] = pool
	log.Printf("ResourcePool %s registered as active", (*pool).GetID())
}

// UnregisterActivePool elimina un ResourcePool de la lista de activos
func (m *ResourcePoolManager) UnregisterActivePool(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.activePools[id]; !exists {
		return fmt.Errorf("resource pool %s not found", id)
	}
	delete(m.activePools, id)
	log.Printf("ResourcePool %s unregistered", id)
	return nil
}

// GetActivePool devuelve un pool activo por su ID
func (m *ResourcePoolManager) GetActivePool(id string) (*ports.ResourcePool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool, exists := m.activePools[id]
	return pool, exists
}

// ListActivePools devuelve todos los pools activos
func (m *ResourcePoolManager) ListActivePools() []*ports.ResourcePool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pools := make([]*ports.ResourcePool, 0, len(m.activePools))
	for _, pool := range m.activePools {
		pools = append(pools, pool)
	}
	return pools
}

// FindActivePoolsByType busca pools activos por su tipo
func (m *ResourcePoolManager) FindActivePoolsByType(poolType string) []*ports.ResourcePool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*ports.ResourcePool
	for _, pool := range m.activePools {
		// Podemos añadir una función GetType() a la interfaz ResourcePool
		// O intentar inferir el tipo a través de otras propiedades
		config, err := m.GetConfig((*pool).GetID())
		if err != nil {
			continue
		}

		if configType, ok := config["type"].(string); ok && configType == poolType {
			result = append(result, pool)
		}
	}
	return result
}

// GetActivePoolsCount devuelve el número total de pools activos
func (m *ResourcePoolManager) GetActivePoolsCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.activePools)
}

// GetStats obtiene estadísticas agregadas de todos los pools activos
func (m *ResourcePoolManager) GetStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// Total de pools activos
	stats["totalActivePools"] = m.GetActivePoolsCount()

	// Contar por tipo
	typeCounts := make(map[string]int)
	for _, pool := range m.ListActivePools() {
		config, err := m.GetConfig((*pool).GetID())
		if err != nil {
			continue
		}

		if poolType, ok := config["type"].(string); ok {
			typeCounts[poolType]++
		}
	}
	stats["poolsByType"] = typeCounts

	return stats
}
