package factories

import (
	"context"
	"fmt"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"time"

	"dev.rubentxu.devops-platform/worker/config"

	"dev.rubentxu.devops-platform/worker/internal/adapters/grpc"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"dev.rubentxu.devops-platform/worker/internal/ports"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sWorker implementa WorkerInstance para ejecutar un Pod que contenga el servidor gRPC.
type K8sWorker struct {
	task       domain.TaskExecution
	endpoint   *domain.WorkerEndpoint
	grpcConfig config.GRPCConfig
	k8sCfg     config.K8sConfig
	clientset  *kubernetes.Clientset
}

func (k *K8sWorker) GetID() string {
	//TODO implement me
	panic("implement me")
}

func (k *K8sWorker) GetName() string {
	//TODO implement me
	panic("implement me")
}

func (k *K8sWorker) GetType() string {
	//TODO implement me
	panic("implement me")
}

// NewK8sWorker crea una instancia de K8sWorker con la misma firma que DockerWorker
func NewK8sWorker(task domain.TaskExecution, grpcConfig config.GRPCConfig, k8sCfg config.K8sConfig) (ports.WorkerInstance, error) {
	// Crear el clientset de Kubernetes
	cs, err := getK8sClient(k8sCfg)
	if err != nil {
		return nil, fmt.Errorf("error creando clientset K8s: %v", err)
	}
	return &K8sWorker{
		task:       task,
		grpcConfig: grpcConfig,
		k8sCfg:     k8sCfg,
		clientset:  cs,
	}, nil
}

// Start crea el Pod en Kubernetes y espera a que esté en Running, luego setea k.endpoint
func (k *K8sWorker) Start(ctx context.Context, outputChan chan<- domain.ProcessOutput) (*domain.WorkerEndpoint, error) {
	log.Printf("Iniciando K8sWorker con spec=%v", k.task.WorkerSpec)

	// Determinar la imagen
	workerImage := k.task.WorkerSpec.Image
	if workerImage == "" {
		k.sendErrorMessage(outputChan, "No se definió imagen para el Pod")
		workerImage = k.k8sCfg.DefaultImage
	}

	// Armar la espec del Pod
	podName := fmt.Sprintf("task-%s", k.task.Name)
	podSpec := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        podName,
			Namespace:   k.k8sCfg.Namespace,
			Labels:      k.k8sCfg.Labels,
			Annotations: k.k8sCfg.Annotations,
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:  "grpc-worker-container",
					Image: workerImage,
					Ports: []apiv1.ContainerPort{
						{
							Name:          "grpc",
							ContainerPort: 50051,
							Protocol:      apiv1.ProtocolTCP,
						},
					},
					Env: buildK8sEnvVars(k.task.WorkerSpec.Env),
					// Args/Command según necesites (o se lo dejas al servidor gRPC)
				},
			},
			RestartPolicy: apiv1.RestartPolicyNever,
		},
	}

	// Crear el Pod en el cluster
	_, err := k.clientset.CoreV1().Pods("default").Create(ctx, podSpec, metav1.CreateOptions{})
	if err != nil {
		k.sendErrorMessage(outputChan, "Error creando Pod")
		return nil, fmt.Errorf("error creando Pod %s: %v", podName, err)
	}
	log.Printf("Pod %s creado en Kubernetes", podName)

	// Esperar a que el Pod entre en Running
	err = waitForPodRunning(ctx, k.clientset, podName, "default")
	if err != nil {
		k.sendErrorMessage(outputChan, "Error esperando que el Pod esté en Running")
		return nil, fmt.Errorf("error esperando que el Pod %s esté en Running: %v", podName, err)
	}
	log.Printf("Pod %s está en Running", podName)

	// Obtener información del Pod (IP, etc.)
	pod, err := k.clientset.CoreV1().Pods("default").Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		k.sendErrorMessage(outputChan, "Error obteniendo Pod")
		return nil, fmt.Errorf("error obteniendo Pod %s: %v", podName, err)
	}

	podIP := pod.Status.PodIP
	if podIP == "" {
		k.sendErrorMessage(outputChan, "No se encontró la IP del Pod")
		return nil, fmt.Errorf("no se encontró la IP del Pod %s", podName)
	}

	// Configurar endpoint
	k.endpoint = &domain.WorkerEndpoint{
		WorkerID: k.task.Name,
		Address:  podIP,   // IP interna del Pod
		Port:     "50051", // Puerto del contenedor gRPC
	}

	log.Printf("K8sWorkerEndpoint: IP=%s Puerto=%s", k.endpoint.Address, k.endpoint.Port)
	return k.endpoint, nil
}

// sendErrorMessage reenvía un mensaje de error al outputChan si está disponible
func (k *K8sWorker) sendErrorMessage(outputChan chan<- domain.ProcessOutput, errMsg string) {
	if outputChan == nil {
		return
	}
	outputChan <- domain.ProcessOutput{
		IsError:   true,
		Output:    errMsg,
		ProcessID: k.task.ID.String(),
	}
}

// Run crea un cliente gRPC y llama StartProcess, enviando la salida a outputChan
func (k *K8sWorker) Run(ctx context.Context, t domain.TaskExecution, outputChan chan<- domain.ProcessOutput) error {
	grpcClient, err := k.createGRPCClient()
	if err != nil {
		return fmt.Errorf("error creando cliente gRPC: %w", err)
	}
	defer grpcClient.Close()

	cmds := k.task.WorkerSpec.Command
	if len(cmds) == 0 {
		cmds = []string{"echo", "Hola desde K8sWorker"}
	}
	envMap := k.task.WorkerSpec.Env
	if envMap == nil {
		envMap = map[string]string{}
	}

	// Llamar al proceso remoto (StartProcess)
	if err := grpcClient.StartProcess(ctx, t.ID.String(), cmds, envMap, k.task.WorkerSpec.WorkingDir, outputChan); err != nil {
		return fmt.Errorf("error en StartProcess: %v", err)
	}

	return nil
}

// Stop llama a StopProcess gRPC (o elimina el Pod, según sea tu necesidad)
func (k *K8sWorker) Stop(ctx context.Context) (bool, string, error) {
	grpcClient, err := k.createGRPCClient()
	if err != nil {
		return false, "", fmt.Errorf("error creando gRPC client para Stop: %w", err)
	}
	defer grpcClient.Close()

	success, msg, err := grpcClient.StopProcess(ctx, k.endpoint.WorkerID)
	if err != nil {
		return false, msg, fmt.Errorf("error en StopProcess (K8s): %v", err)
	}
	log.Printf("Stop K8s process success=%v, msg=%v", success, msg)

	// En caso quieras eliminar el Pod:
	// err = k.clientset.CoreV1().Pods("default").Delete(ctx, "task-"+k.task.Name, metav1.DeleteOptions{})
	// if err != nil { ... }

	return success, msg, nil
}

// StartMonitoring inicia la monitorización de salud
func (k *K8sWorker) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error {
	grpcClient, err := k.createGRPCClient()
	if err != nil {
		return fmt.Errorf("error creando gRPC client para StartMonitoring: %w", err)
	}
	// No cerramos grpcClient aquí si necesitamos monitorizar en segundo plano
	err = grpcClient.MonitorHealth(ctx, k.endpoint.WorkerID, checkInterval, healthChan)
	if err != nil {
		return fmt.Errorf("error en MonitorHealth de K8s: %v", err)
	}
	return nil
}

// GetEndpoint retorna el WorkerEndpoint (Pod IP y puerto)
func (k *K8sWorker) GetEndpoint() *domain.WorkerEndpoint {
	return k.endpoint
}

// createGRPCClient construye el RPSClient con la configuración TLS o token, tal como docker_instance.go
func (k *K8sWorker) createGRPCClient() (*grpc.RPSClient, error) {
	if k.endpoint == nil {
		return nil, fmt.Errorf("endpoint no inicializado en K8sWorker")
	}
	rpcClientConfig := &grpc.RemoteProcessClientConfig{
		Address:    fmt.Sprintf("%s:%s", k.endpoint.Address, k.endpoint.Port),
		ClientCert: k.grpcConfig.ClientCertPath,
		ClientKey:  k.grpcConfig.ClientKeyPath,
		CACert:     k.grpcConfig.CACertPath,
		AuthToken:  k.grpcConfig.JWTToken,
	}
	rpcClient, err := grpc.New(rpcClientConfig)
	if err != nil {
		return nil, fmt.Errorf("error creando remote_process_client K8s: %v", err)
	}
	return rpcClient, nil
}

// getK8sClient crea un clientset de Kubernetes para interactuar con la API
func getK8sClient(k8sCfg config.K8sConfig) (*kubernetes.Clientset, error) {
	if k8sCfg.InCluster {
		// Worker corre dentro del cluster
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, err
		}
		return kubernetes.NewForConfig(cfg)
	}

	// Si no estás en cluster:
	if k8sCfg.KubeConfig == "" {
		return nil, fmt.Errorf("no se definió K8S_CONFIG_PATH y K8S_IN_CLUSTER=false")
	}

	cfg, err := clientcmd.BuildConfigFromFlags("", k8sCfg.KubeConfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
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

// buildK8sEnvVars convierte tu map[string]string en []apiv1.EnvVar
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
