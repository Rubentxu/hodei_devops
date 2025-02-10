package resources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/domain"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc" // Import for OIDC authentication
)

type KubernetesProvider struct {
	BaseProvider
	client           *kubernetes.Clientset
	metricsClient    *metrics.Clientset
	defaultNamespace string
	pools            map[string]*KubernetesPool // Cache of Kubernetes pools
	mutex            sync.RWMutex
}

type KubernetesPool struct {
	ID          string
	Namespace   string
	Constraints domain.ResourceConstraints
	Labels      map[string]string
}

func (k *KubernetesProvider) init() {
	k.technologyName = "kubernetes"
	k.supportedResources = []domain.ResourceType{domain.CPU, domain.Memory, domain.Storage, domain.Instances}
}

func newKubernetesProvider(config ProviderConfig) (*KubernetesProvider, error) {
	var restConfig *rest.Config
	var err error

	// Check for in-cluster config first
	restConfig, err = rest.InClusterConfig()
	if err != nil {
		// Fallback to kubeconfig if in-cluster config fails
		kubeconfigPath, ok := config["kubeconfig"].(string)
		if !ok || kubeconfigPath == "" {
			kubeconfigPath = "~/.kube/config" // Default kubeconfig path
		}

		restConfig, err = loadKubeconfig(kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load Kubernetes config: %v", err)
		}
	}

	// Create Kubernetes client
	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	// Create Metrics client
	metricsClient, err := metrics.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes Metrics client: %v", err)
	}

	// Get default namespace from config, or default to "default"
	defaultNamespace, ok := config["defaultNamespace"].(string)
	if !ok {
		defaultNamespace = "default"
	}

	return &KubernetesProvider{
		client:           client,
		metricsClient:    metricsClient,
		defaultNamespace: defaultNamespace,
		pools:            make(map[string]*KubernetesPool),
		mutex:            sync.RWMutex{},
	}, nil
}

// Helper function to load kubeconfig from a given path
func loadKubeconfig(kubeconfigPath string) (*rest.Config, error) {
	// Expand tilde (~) in path
	if strings.HasPrefix(kubeconfigPath, "~/") {
		homeDir, _ := os.UserHomeDir()
		kubeconfigPath = filepath.Join(homeDir, kubeconfigPath[2:])
	}

	// Load config from the specified kubeconfig file
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (k *KubernetesProvider) CreatePool(ctx context.Context, pool *domain.ResourcePool) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	// In Kubernetes, a "pool" can be thought of as a namespace with resource quotas.
	// Check if the namespace already exists
	_, err := k.client.CoreV1().Namespaces().Get(ctx, pool.ID, metav1.GetOptions{})
	if err == nil {
		return fmt.Errorf("namespace (pool) %s already exists", pool.ID)
	}

	// Create the namespace
	namespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: pool.ID,
		},
	}
	_, err = k.client.CoreV1().Namespaces().Create(ctx, namespace, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	// Optionally, create a ResourceQuota for the namespace based on pool.Constraints
	if len(pool.Constraints) > 0 {
		resourceQuota := &v1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      pool.ID + "-quota",
				Namespace: pool.ID,
			},
			Spec: v1.ResourceQuotaSpec{
				Hard: k.parseResourceConstraints(pool.Constraints),
			},
		}
		_, err = k.client.CoreV1().ResourceQuotas(pool.ID).Create(ctx, resourceQuota, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	// Cache the created pool
	k.pools[pool.ID] = &KubernetesPool{
		ID:          pool.ID,
		Namespace:   pool.ID,
		Constraints: pool.Constraints,
	}

	return nil
}

func (k *KubernetesProvider) DeletePool(ctx context.Context, poolID string) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	// In Kubernetes, deleting a "pool" means deleting a namespace.
	err := k.client.CoreV1().Namespaces().Delete(ctx, poolID, metav1.DeleteOptions{})
	if err != nil {
		return err
	}

	// Remove the pool from the cache
	delete(k.pools, poolID)
	return nil
}

func (k *KubernetesProvider) GetPool(ctx context.Context, poolID string) (*domain.ResourcePool, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	// Check the cache first
	if cachedPool, ok := k.pools[poolID]; ok {
		return &domain.ResourcePool{
			ID:          cachedPool.ID,
			Name:        cachedPool.Namespace,
			Technology:  k.GetTechnologyName(),
			Constraints: cachedPool.Constraints,
		}, nil
	}

	// If not in cache, retrieve the namespace from Kubernetes
	namespace, err := k.client.CoreV1().Namespaces().Get(ctx, poolID, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Convert Kubernetes namespace to domain.ResourcePool
	pool := &domain.ResourcePool{
		ID:         namespace.Name,
		Name:       namespace.Name,
		Technology: k.GetTechnologyName(),
	}

	// Retrieve resource quota for the namespace, if any
	quota, err := k.client.CoreV1().ResourceQuotas(poolID).Get(ctx, poolID+"-quota", metav1.GetOptions{})
	if err == nil {
		pool.Constraints = k.convertResourceListToConstraints(quota.Spec.Hard)
	}

	// Cache the pool
	k.mutex.Lock()
	k.pools[poolID] = &KubernetesPool{
		ID:          pool.ID,
		Namespace:   pool.Name,
		Constraints: pool.Constraints,
	}
	k.mutex.Unlock()

	return pool, nil
}

func (k *KubernetesProvider) GetPoolUsage(ctx context.Context, poolID string) (map[domain.ResourceType]domain.ResourceUsage, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	// Check if the pool exists
	_, ok := k.pools[poolID]
	if !ok {
		// If not in cache, try fetching from Kubernetes
		_, err := k.client.CoreV1().Namespaces().Get(ctx, poolID, metav1.GetOptions{})
		if err != nil {
			return nil, fmt.Errorf("pool (namespace) %s not found", poolID)
		}

		// Update cache
		k.mutex.Lock()
		k.pools[poolID] = &KubernetesPool{
			ID:        poolID,
			Namespace: poolID,
		}
		k.mutex.Unlock()
	}

	// Get the Pod Metrics List in the namespace
	podMetricsList, err := k.metricsClient.MetricsV1beta1().PodMetricses(poolID).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	usage := make(map[domain.ResourceType]domain.ResourceUsage)
	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
			for resType, quantity := range container.Usage {
				switch domain.ResourceType(resType) {
				case domain.CPU, domain.Memory:
					resourceUsage, ok := usage[domain.ResourceType(resType)]
					if !ok {
						resourceUsage = domain.ResourceUsage{
							Timestamp: time.Now(),
						}
					}
					used, _ := resource.ParseQuantity(resourceUsage.Used)
					newUsed := quantity.DeepCopy()
					newUsed.Add(used)
					resourceUsage.Used = newUsed.String()
					usage[domain.ResourceType(resType)] = resourceUsage
				}
			}
		}
	}

	// Get resource quota to calculate Max and Available, if they exist
	quota, err := k.client.CoreV1().ResourceQuotas(poolID).Get(ctx, poolID+"-quota", metav1.GetOptions{})
	if err == nil {
		for resType, quantity := range quota.Status.Hard {
			resourceUsage, ok := usage[domain.ResourceType(resType)]
			if !ok {
				resourceUsage = domain.ResourceUsage{
					Timestamp: time.Now(),
				}
			}
			resourceUsage.Max = quantity.String()

			usedQuantity, _ := resource.ParseQuantity(resourceUsage.Used)
			availableQuantity := quantity.DeepCopy()
			availableQuantity.Sub(usedQuantity)
			resourceUsage.Available = availableQuantity.String()

			usage[domain.ResourceType(resType)] = resourceUsage
		}
	}

	return usage, nil
}

func (k *KubernetesProvider) GetResourceUsage(ctx context.Context, poolID string, resourceType domain.ResourceType) (domain.ResourceUsage, error) {
	k.mutex.RLock()
	defer k.mutex.RUnlock()

	// Check if the pool exists
	_, ok := k.pools[poolID]
	if !ok {
		// If not in cache, try fetching from Kubernetes
		_, err := k.client.CoreV1().Namespaces().Get(ctx, poolID, metav1.GetOptions{})
		if err != nil {
			return domain.ResourceUsage{}, fmt.Errorf("pool (namespace) %s not found", poolID)
		}

		// Update cache
		k.mutex.Lock()
		k.pools[poolID] = &KubernetesPool{
			ID:        poolID,
			Namespace: poolID,
		}
		k.mutex.Unlock()
	}

	// Get the Pod Metrics List in the namespace
	podMetricsList, err := k.metricsClient.MetricsV1beta1().PodMetricses(poolID).List(ctx, metav1.ListOptions{})
	if err != nil {
		return domain.ResourceUsage{}, err
	}

	var used resource.Quantity
	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
			if quantity, ok := container.Usage[v1.ResourceName(resourceType)]; ok {
				used.Add(quantity)
			}
		}
	}

	resourceUsage := domain.ResourceUsage{
		Used:      used.String(),
		Timestamp: time.Now(),
	}

	// Get resource quota to calculate Max and Available, if they exist
	quota, err := k.client.CoreV1().ResourceQuotas(poolID).Get(ctx, poolID+"-quota", metav1.GetOptions{})
	if err == nil {
		if quantity, ok := quota.Status.Hard[v1.ResourceName(resourceType)]; ok {
			resourceUsage.Max = quantity.String()
			available := quantity.DeepCopy()
			available.Sub(used)
			resourceUsage.Available = available.String()
		}
	}

	return resourceUsage, nil
}

func (k *KubernetesProvider) SetConstraints(ctx context.Context, poolID string, constraints domain.ResourceConstraints) error {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	// Check if the namespace (pool) exists
	_, err := k.client.CoreV1().Namespaces().Get(ctx, poolID, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("namespace (pool) %s not found: %v", poolID, err)
	}

	// Create or update the ResourceQuota for the namespace
	resourceQuota := &v1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      poolID + "-quota",
			Namespace: poolID,
		},
		Spec: v1.ResourceQuotaSpec{
			Hard: k.parseResourceConstraints(constraints),
		},
	}

	// Try to get the existing ResourceQuota
	existingQuota, err := k.client.CoreV1().ResourceQuotas(poolID).Get(ctx, resourceQuota.Name, metav1.GetOptions{})
	if err != nil {
		// If the ResourceQuota does not exist, create it
		_, err = k.client.CoreV1().ResourceQuotas(poolID).Create(ctx, resourceQuota, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ResourceQuota: %v", err)
		}
	} else {
		// If the ResourceQuota exists, update it
		existingQuota.Spec.Hard = resourceQuota.Spec.Hard
		_, err = k.client.CoreV1().ResourceQuotas(poolID).Update(ctx, existingQuota, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ResourceQuota: %v", err)
		}
	}

	// Update the cache with new constraints
	if pool, ok := k.pools[poolID]; ok {
		pool.Constraints = constraints
	} else {
		k.pools[poolID] = &KubernetesPool{
			ID:          poolID,
			Namespace:   poolID,
			Constraints: constraints,
		}
	}

	return nil
}

func (k *KubernetesProvider) GetSupportedResources() []domain.ResourceType {
	return k.supportedResources
}

func (k *KubernetesProvider) GetTechnologyName() string {
	return k.technologyName
}

func (k *KubernetesProvider) GetChildPools(ctx context.Context, poolID string) ([]*domain.ResourcePool, error) {
	// In this basic implementation, Kubernetes doesn't have a direct concept of nested namespaces.
	// All namespaces are considered as "root" pools.
	// If you need to simulate hierarchy, consider using labels or a custom CRD.

	if poolID != "" {
		return nil, nil // No child pools for a given namespace
	}

	namespaces, err := k.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var pools []*domain.ResourcePool
	for _, ns := range namespaces.Items {
		pool := &domain.ResourcePool{
			ID:           ns.Name,
			Name:         ns.Name,
			Technology:   k.GetTechnologyName(),
			ParentPoolID: "", // No parent in this basic implementation
		}

		// Retrieve resource quota for the namespace, if any
		quota, err := k.client.CoreV1().ResourceQuotas(ns.Name).Get(ctx, ns.Name+"-quota", metav1.GetOptions{})
		if err == nil {
			pool.Constraints = k.convertResourceListToConstraints(quota.Spec.Hard)
		}

		pools = append(pools, pool)
	}

	return pools, nil
}

// parseResourceConstraints converts domain.ResourceConstraints to Kubernetes resource.Quantity
func (k *KubernetesProvider) parseResourceConstraints(constraints domain.ResourceConstraints) v1.ResourceList {
	resourceList := make(v1.ResourceList)

	for resType, constraintValue := range constraints {
		quantity, err := resource.ParseQuantity(constraintValue)
		if err == nil {
			switch resType {
			case domain.CPU:
				resourceList[v1.ResourceCPU] = quantity
			case domain.Memory:
				resourceList[v1.ResourceMemory] = quantity
			case domain.Storage:
				resourceList[v1.ResourceStorage] = quantity
			case domain.Instances:
				resourceList[v1.ResourcePods] = quantity
			}
		}
	}

	return resourceList
}

func (k *KubernetesProvider) convertResourceListToConstraints(resourceList v1.ResourceList) domain.ResourceConstraints {
	constraints := make(domain.ResourceConstraints)

	for resType, quantity := range resourceList {
		switch resType {
		case v1.ResourceCPU:
			constraints[domain.CPU] = quantity.String()
		case v1.ResourceMemory:
			constraints[domain.Memory] = quantity.String()
		case v1.ResourceStorage:
			constraints[domain.Storage] = quantity.String()
		case v1.ResourcePods:
			constraints[domain.Instances] = quantity.String()
		}
	}

	return constraints
}
