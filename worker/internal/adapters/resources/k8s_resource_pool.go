package resources

import (
	"context"
	"dev.rubentxu.devops-platform/worker/config"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"time"

	v1 "k8s.io/api/core/v1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/client-go/kubernetes"
	//"k8s.io/client-go/rest"
	//
	//"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	//metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
)

type KubernetesResourcePool struct {
	clientset     *kubernetes.Clientset
	metricsClient *metricsclientset.Clientset
	config        config.K8sConfig
}

func NewKubernetesResourcePool(config config.K8sConfig) (ports.ResourcePool, error) {
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

	return &KubernetesResourcePool{clientset: clientset, metricsClient: metricsClient, config: config}, nil
}

func (k *KubernetesResourcePool) GetConfig() any {
	return k.config // Devolver la configuración de Kubernetes
}

func (k *KubernetesResourcePool) monitorTask(ctx context.Context, taskExecution *domain.TaskExecution, namespace string) {
	timeout := time.After(5 * time.Minute)
	podsClient := k.clientset.CoreV1().Pods(namespace) // Usar el namespace

	watcher, err := podsClient.Watch(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", taskExecution.WorkerID), //Usar el workerID que es el nombre
	})
	if err != nil {
		fmt.Printf("error watching task: %v\n", err)
		return
	}
	defer watcher.Stop()

	for {
		select {
		case <-timeout:
			now := time.Now()
			taskExecution.FinishTime = &now
			taskExecution.State = domain.Failed
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return
			}

			pod, ok := event.Object.(*apiv1.Pod)
			if !ok {
				continue
			}

			switch pod.Status.Phase {
			case apiv1.PodSucceeded:
				now := time.Now()
				taskExecution.FinishTime = &now
				taskExecution.State = domain.Completed
				if svc, err := k.clientset.CoreV1().Services(namespace).Get(ctx, taskExecution.WorkerID+"-service", metav1.GetOptions{}); err == nil {
					for _, port := range svc.Spec.Ports {
						taskExecution.HostPorts = append(taskExecution.HostPorts, fmt.Sprintf("%s:%d", svc.Spec.ClusterIP, port.Port))
					}
					taskExecution.Status.Endpoint = &domain.WorkerEndpoint{
						Address: svc.Spec.ClusterIP,                        // ClusterIP del servicio
						Port:    fmt.Sprintf("%d", svc.Spec.Ports[0].Port), // Primer puerto (simplificación)
					}
				}
				return
			case apiv1.PodFailed:
				now := time.Now()
				taskExecution.FinishTime = &now
				taskExecution.State = domain.Failed
				return
			case apiv1.PodRunning:
				taskExecution.State = domain.Running
				for _, container := range pod.Spec.Containers {
					for _, port := range container.Ports {
						taskExecution.HostPorts = append(taskExecution.HostPorts, fmt.Sprintf("%s:%d", pod.Status.PodIP, port.ContainerPort))
					}
				}
				taskExecution.Status.Endpoint = &domain.WorkerEndpoint{
					Address: pod.Status.PodIP,                                                 // IP del Pod
					Port:    fmt.Sprintf("%d", pod.Spec.Containers[0].Ports[0].ContainerPort), // Primer puerto del primer contenedor (simplificación)
				}
			}
		}
	}
}

func (k *KubernetesResourcePool) GetStats() (*domain.Stats, error) {
	ctx := context.Background()
	namespace := k.config.Namespace // Usar el namespace de la configuración.
	if namespace == "" {
		namespace = apiv1.NamespaceDefault // Namespace por defecto.
	}

	// Obtener todos los Pods en todos los namespaces.  Podrías filtrar por namespace si fuera necesario.
	pods, err := k.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	stats := &domain.Stats{
		MemStats:  &domain.MemInfo{},
		DiskStats: &domain.Disk{},
		CpuStats:  &domain.CPUStat{},
		LoadStats: &domain.LoadAvg{},
		TaskCount: len(pods.Items), // Contar Pods
	}

	// Obtener métricas de los Pods
	podMetricsList, err := k.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Si no puedes obtener métricas, continúa, pero registra el error (podrías querer ser más estricto aquí)
		fmt.Printf("failed to get pod metrics: %v\n", err)
		//return nil, fmt.Errorf("failed to get pod metrics: %w", err) //O retornar error en vez de imprimir
	} else {
		k.aggregatePodMetrics(stats, podMetricsList)
	}

	// Obtener información de los nodos para el disco, pero solo de los nodos en este namespace.
	nodes, err := k.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	k.aggregateNodeStats(stats, nodes)

	return stats, nil
}

func (k *KubernetesResourcePool) aggregatePodMetrics(stats *domain.Stats, podMetricsList *v1beta1.PodMetricsList) {
	for _, podMetrics := range podMetricsList.Items {
		for _, container := range podMetrics.Containers {
			// CPU:  La métrica es en "nanocores", convierte a unidades de CPU completas (dividiendo por 1e9)
			cpuUsage := container.Usage.Cpu().MilliValue() //o .Value si quieres mas presicion

			// Memoria:  La métrica está en bytes.
			memUsage := container.Usage.Memory().Value()

			stats.MemStats.MemAvailable += uint64(memUsage)

			stats.CpuStats.User += uint64(cpuUsage) // Simplificación:  Todo el uso a "user".

		}
	}
}

func (k *KubernetesResourcePool) aggregateNodeStats(stats *domain.Stats, nodes *v1.NodeList) {
	for _, node := range nodes.Items {

		// Capacidad total de memoria del nodo.
		memTotal := node.Status.Capacity.Memory().Value()
		stats.MemStats.MemTotal += uint64(memTotal)

		// Capacidad total de CPU del nodo (en milicores, como MilliValue()).
		cpuCapacity := node.Status.Capacity.Cpu().MilliValue()
		stats.CpuStats.System += uint64(cpuCapacity) //Simplificacion poner todo en system

		//Espacio de disco, se debe obtener del nodo del contenedor.
		ephemeralStorageCapacity := node.Status.Capacity.StorageEphemeral().Value()
		stats.DiskStats.All += uint64(ephemeralStorageCapacity)

		//Asignado, es decir espacio que ya se solicito
		ephemeralStorageAllocatable := node.Status.Allocatable.StorageEphemeral().Value()

		stats.DiskStats.Free += uint64(ephemeralStorageAllocatable)
		stats.DiskStats.Used += uint64(ephemeralStorageCapacity - ephemeralStorageAllocatable)

	}
}

func (k *KubernetesResourcePool) Matches(task domain.Task) bool {
	// Implementación básica.  ¡Ajusta esto a tu WorkerSpec real!
	// Por ejemplo, podrías verificar si hay recursos suficientes,
	// o si el namespace existe.

	// Comprobación básica (ejemplo, debe adaptarse a tu WorkerSpec)
	if task.WorkerSpec.Type != "kubernetes" {
		return false
	}

	//Comprobaciones adicionales si es necesario

	return true
}
