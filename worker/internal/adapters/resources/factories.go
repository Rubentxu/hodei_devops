package resources

import (
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"fmt"
	"github.com/docker/docker/client"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"os"
	"path/filepath"
	"strings"

	"dev.rubentxu.devops-platform/worker/internal/ports"
	"k8s.io/client-go/tools/clientcmd"
)

// ************** Provider Factory **************

type ProviderConfig map[string]interface{}

type ProviderFactory struct {
	providers map[string]func(ProviderConfig) (ports.ResourcePoolProvider, error)
}

func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{
		providers: make(map[string]func(ProviderConfig) (ports.ResourcePoolProvider, error)),
	}
}

// RegisterProviderType adds a new type of provider to the factory
func (f *ProviderFactory) RegisterProviderType(
	name string,
	creatorFunc func(ProviderConfig) (ports.ResourcePoolProvider, error),
) {
	f.providers[name] = creatorFunc
}

// CreateProvider instantiates a provider based on the requested technology
func (f *ProviderFactory) CreateProvider(
	techName string,
	config ProviderConfig,
) (ports.ResourcePoolProvider, error) {
	creatorFunc, exists := f.providers[techName]
	if !exists {
		return nil, fmt.Errorf("provider type '%s' not registered", techName)
	}

	return creatorFunc(config)
}

// ************** Provider Registrations **************

func InitDockerProvider(config ProviderConfig) (ports.ResourcePoolProvider, error) {
	// Parse Docker-specific configuration
	endpoint, ok := config["endpoint"].(string)
	if !ok {
		endpoint = "unix:///var/run/docker.sock"
	}

	// Create Docker client
	cli, err := client.NewClientWithOpts(
		client.WithHost(endpoint),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}

	provider := &DockerProvider{
		client: cli,
		pools:  make(map[string]*DockerPool),
	}
	provider.init()
	return provider, nil
}

func InitKubernetesProvider(config ProviderConfig) (ports.ResourcePoolProvider, error) {
	provider, err := newKubernetesProvider(config)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes provider: %v", err)
	}
	provider.init()
	return provider, err
}

// ************** Base Provider (Optional) **************

type BaseProvider struct {
	technologyName     string
	supportedResources []domain.ResourceType
}

func (b *BaseProvider) GetTechnologyName() string {
	return b.technologyName
}

func (b *BaseProvider) GetSupportedResources() []domain.ResourceType {
	return b.supportedResources
}

// ************** Provider Registrations **************

func initDockerProvider(config ProviderConfig) (ports.ResourcePoolProvider, error) {
	// Parsear configuración específica de Docker
	endpoint, ok := config["endpoint"].(string)
	if !ok {
		endpoint = "unix:///var/run/docker.sock"
	}

	// Crear cliente Docker
	cli, err := client.NewClientWithOpts(
		client.WithHost(endpoint),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, err
	}

	return &DockerProvider{
		client: cli,
		pools:  make(map[string]*DockerPool),
	}, nil
}

func initKubernetesProvider(config ProviderConfig) (ports.ResourcePoolProvider, error) {
	// Configuración para Kubernetes
	kubeconfig, _ := config["kubeconfig"].(string)
	namespace, _ := config["defaultNamespace"].(string)

	// Crear cliente Kubernetes
	var cli *kubernetes.Clientset
	var err error

	if kubeconfig == "" {
		cli, err = kubernetes.NewForConfig(rest.InClusterConfig())
	} else {
		cli, err = loadKubeconfig(kubeconfig)
	}

	if err != nil {
		return nil, err
	}

	return &KubernetesProvider{
		client:           cli,
		defaultNamespace: namespace,
	}, nil
}

// ************** Uso de la Factory **************

func initialize() {
	factory := NewProviderFactory()

	// Registrar todos los proveedores disponibles
	factory.RegisterProviderType("docker", initDockerProvider)
	factory.RegisterProviderType("kubernetes", initKubernetesProvider)

	// Ejemplo: Crear un provider Docker
	dockerConfig := ProviderConfig{
		"endpoint": "tcp://docker-host:2375",
	}

	dockerProvider, err := factory.CreateProvider("docker", dockerConfig)
	if err != nil {
		panic(err)
	}

	// Ejemplo: Crear un provider Kubernetes
	k8sConfig := ProviderConfig{
		"kubeconfig":       "/path/to/kubeconfig",
		"defaultNamespace": "production",
	}

	k8sProvider, err := factory.CreateProvider("kubernetes", k8sConfig)
	if err != nil {
		panic(err)
	}

	// Registrar los providers en el ResourceManager
	manager := NewResourceManager()
	manager.RegisterProvider(dockerProvider)
	manager.RegisterProvider(k8sProvider)

	// Usar el manager normalmente...
}
