package worker_instance

import (
	"context"
	"fmt"
	"log"
	"time"

	"dev.rubentxu.devops-platform/worker/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sWorkerInstance implementa WorkerInstance para Kubernetes
type K8sWorkerInstance struct {
	rpcClient    *grpc.Client
	instanceSpec domain.WorkerInstanceSpec
	workerID     *domain.WorkerID
}

func NewK8sWorkerInstance(config domain.WorkerInstanceSpec, rpcClient *grpc.Client) ports.WorkerInstance {
	return &K8sWorkerInstance{
		instanceSpec: config,
		rpcClient:    rpcClient,
	}
}

func getK8sClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func waitForPodRunning(ctx context.Context, clientset *kubernetes.Clientset, podName, namespace string) error {
	return wait.PollUntilContextCancel(ctx, 2*time.Second, false, func(ctx context.Context) (bool, error) {
		pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			log.Printf("Error obteniendo Pod %s: %v", podName, err)
			return false, nil
		}
		if pod.Status.Phase == apiv1.PodRunning {
			log.Printf("Pod %s está en Running", podName)
			return true, nil
		}
		log.Printf("Esperando a que el Pod %s llegue a Running. Actual: %s", podName, pod.Status.Phase)
		return false, nil
	})
}

func (k *K8sWorkerInstance) Start(ctx context.Context) (*domain.WorkerID, error) {
	clientset, err := getK8sClient()
	if err != nil {
		return nil, fmt.Errorf("error creando K8s client: %v", err)
	}

	// Crear Pod
	podName := fmt.Sprintf("k8s-worker-%s", k.instanceSpec.Name)
	image := k.instanceSpec.Image
	if image == "" {
		image = "your-k8s-grpc-image:latest"
	}

	podSpec := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: "default",
			Labels: map[string]string{
				"app": "grpc-worker",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "grpc-worker-container",
					Image: image,
					Ports: []apiv1.ContainerPort{
						{
							Name:          "grpc",
							ContainerPort: 50051,
							Protocol:      apiv1.ProtocolTCP,
						},
					},
					Env: buildK8sEnvVars(k.instanceSpec.Env),
				},
			},
			RestartPolicy: apiv1.RestartPolicyNever,
		},
	}

	_, err = clientset.CoreV1().Pods("default").Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creando Pod %s: %v", podName, err)
	}

	err = waitForPodRunning(ctx, clientset, podName, "default")
	if err != nil {
		return nil, fmt.Errorf("error esperando a que el Pod %s esté en Running: %v", podName, err)
	}

	pod, err := clientset.CoreV1().Pods("default").Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error obteniendo Pod %s: %v", podName, err)
	}

	if pod.Status.PodIP == "" {
		return nil, fmt.Errorf("IP del Pod %s no disponible", podName)
	}

	k.workerID = &domain.WorkerID{
		ID:      podName,
		Address: pod.Status.PodIP,
		Port:    "50051",
	}

	return k.workerID, nil
}

func (k *K8sWorkerInstance) Run(ctx context.Context, t domain.Task, outputChan chan<- *domain.ProcessOutput) domain.TaskResult {
	grpcAddr := fmt.Sprintf("%s:%s", k.workerID.Address, k.workerID.Port)
	newRpcClient, err := grpc.New(&grpc.ClientConfig{
		ServerAddress: grpcAddr,
	})
	if err != nil {
		return domain.TaskResult{Error: fmt.Errorf("error creando cliente gRPC: %v", err)}
	}

	cmds := k.instanceSpec.Command
	if len(cmds) == 0 {
		cmds = []string{"echo", "Hola desde K8sWorkerInstance"}
	}
	envMap := k.instanceSpec.Env
	if envMap == nil {
		envMap = map[string]string{}
	}

	if err := newRpcClient.StartProcess(ctx, t.ID.String(), cmds, envMap, k.instanceSpec.WorkingDir, outputChan); err != nil {
		return domain.TaskResult{Error: fmt.Errorf("error en StartProcess: %v", err)}
	}

	return domain.TaskResult{
		Action: "run",
		Error:  nil,
		Result: "started",
	}
}

func (k *K8sWorkerInstance) Stop() (bool, string, error) {
	// Implementa Stop usando el cliente Kubernetes para eliminar el Pod si es necesario
	return true, "Stopped Pod", nil
}

func (k *K8sWorkerInstance) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error {
	return nil
}

func buildK8sEnvVars(env map[string]string) []apiv1.EnvVar {
	if env == nil {
		return nil
	}
	items := make([]apiv1.EnvVar, 0, len(env))
	for k, v := range env {
		items = append(items, apiv1.EnvVar{Name: k, Value: v})
	}
	return items
}
