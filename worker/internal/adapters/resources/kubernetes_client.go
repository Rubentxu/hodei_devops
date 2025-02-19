package resources

import (
	"fmt"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/ports"
)

// KubernetesClientAdapter adapta el cliente de Kubernetes a la interfaz InfrastructureClient.
type KubernetesClientAdapter struct {
	clientset     *kubernetes.Clientset
	metricsClient *metricsclientset.Clientset
	config        config.K8sConfig
}

func (k *KubernetesClientAdapter) GetConfig() any {
	return k.config
}

func (k *KubernetesClientAdapter) GetNativeClient() any {
	return k.clientset
}

func (k *KubernetesClientAdapter) GetNativeMetricsClient() *metricsclientset.Clientset {
	return k.metricsClient
}

// NewKubernetesClientAdapter crea un KubernetesClientAdapter y lo retorna como una interfaz ResourceIntanceClient.
func NewKubernetesClientAdapter(config config.K8sConfig) (ports.ResourceIntanceClient, error) {
	var kubeConfig *rest.Config
	var err error

	if config.InCluster {
		kubeConfig, err = rest.InClusterConfig()
	} else {
		kubeConfig, err = clientcmd.BuildConfigFromFlags("", config.KubeConfig)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	metricsClient, err := metricsclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Metrics client: %w", err)
	}

	return &KubernetesClientAdapter{clientset: clientset, config: config, metricsClient: metricsClient}, nil
}
