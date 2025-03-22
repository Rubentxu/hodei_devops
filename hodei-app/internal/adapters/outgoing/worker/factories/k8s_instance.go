package factories

import (
	"context"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/incoming/security"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/grpc"
	"dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/resource"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/model"
	"fmt"

	"gopkg.in/yaml.v2"
	"k8s.io/client-go/tools/clientcmd"
	"log"
	"os"
	"time"

	"dev.rubentxu.hodei-devops/hodei-app/config"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"

	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// K8sWorker implementa WorkerInstance para ejecutar un Pod que contenga el servidor gRPC.
type K8sWorker struct {
	execution      model.TaskExecution
	connectionInfo *model.ConnectionInfo
	grpcConfig     config.Config
	k8sCfg         resource.KubernetesResoucesPoolConfig
	clientset      *kubernetes.Clientset
	token          string
}

// NewK8sWorker crea una instancia de K8sWorker con la misma firma que DockerWorker
func NewK8sWorker(execution model.TaskExecution, config config.Config, client ports.ResourceIntanceClient) (ports.WorkerInstance, error) {
	k8sCfg := client.GetConfig().(resource.KubernetesResoucesPoolConfig)
	clientset := client.GetNativeClient().(*kubernetes.Clientset)
	jwtManager := security.NewJWTManager(config.AccessSecret)
	token, err := jwtManager.GenerateToken("admin")
	if err != nil {
		return nil, fmt.Errorf("error generando token JWT: %v", err)
	}
	return &K8sWorker{
		execution:  execution,
		grpcConfig: config,
		k8sCfg:     k8sCfg,
		clientset:  clientset,
		token:      token,
	}, nil
}

func (k *K8sWorker) GetID() model.AggregateID {
	return k.execution.ID
}

func (k *K8sWorker) GetName() string {
	return k.execution.Metadata.Name
}

func (k *K8sWorker) GetType() string {
	return "kubernetes"
}

// loadPodTemplateFromFile carga y parsea un template de Pod desde un archivo YAML
func loadPodTemplateFromFile(templatePath string) (*apiv1.Pod, error) {
	if templatePath == "" {
		return nil, fmt.Errorf("ruta al template de Pod no especificada")
	}

	yamlFile, err := os.ReadFile(templatePath)
	if err != nil {
		return nil, fmt.Errorf("error leyendo archivo template YAML: %w", err)
	}

	podTemplate := &apiv1.Pod{}
	err = yaml.Unmarshal(yamlFile, podTemplate)
	if err != nil {
		return nil, fmt.Errorf("error parseando YAML a Pod: %w", err)
	}

	return podTemplate, nil
}

// Start crea el Pod en Kubernetes y espera a que esté en Running, luego setea k.connectionInfo
func (k *K8sWorker) Start(ctx context.Context, templatePath string, outputChan chan<- model.ProcessOutput) (*model.ConnectionInfo, error) {
	log.Printf("Iniciando K8sWorker con spec=%v y templatePath=%s", k.execution.WorkerDef.Spec, templatePath)

	// Cargar el template del Pod desde el archivo YAML pasado como argumento
	podTemplate, err := loadPodTemplateFromFile(templatePath)
	if err != nil {
		k.sendErrorMessage(outputChan, fmt.Sprintf("Error cargando template de Pod desde '%s': %v", templatePath, err))
		return nil, err
	}

	// **Personalizar el template del Pod**

	// 1. Nombre del Pod
	podName := fmt.Sprintf("execution-%s", k.execution.Metadata.Name)
	podTemplate.ObjectMeta.Name = podName
	log.Printf("Personalizando Pod con nombre: %s", podName)

	// 2. Namespace (tomar de k.k8sCfg.Namespace como valor predeterminado si no está ya en el template)
	if podTemplate.ObjectMeta.Namespace == "" && k.k8sCfg.Namespace != "" {
		podTemplate.ObjectMeta.Namespace = k.k8sCfg.Namespace
		log.Printf("Personalizando Pod con namespace desde config: %s", k.k8sCfg.Namespace)
	} else if podTemplate.ObjectMeta.Namespace != "" {
		log.Printf("Usando namespace del template del Pod: %s", podTemplate.ObjectMeta.Namespace)
	} else {
		log.Println("Namespace no definido ni en template ni en config, usando namespace 'default'") // Podría ser un error según tus requisitos
		podTemplate.ObjectMeta.Namespace = "default"                                                 // o retornar error si el namespace es obligatorio
	}

	// 3. Labels (merge con los definidos en k.k8sCfg.Labels, priorizando los de la config si hay conflicto)
	if k.k8sCfg.Labels != nil {
		if podTemplate.ObjectMeta.Labels == nil {
			podTemplate.ObjectMeta.Labels = make(map[string]string)
		}
		for key, value := range k.k8sCfg.Labels {
			podTemplate.ObjectMeta.Labels[key] = value
		}
		log.Printf("Merge de labels con la config: %v", k.k8sCfg.Labels)
	}

	// 4. Annotations (merge con los definidos en k.k8sCfg.Annotations, similar a labels)
	if k.k8sCfg.Annotations != nil {
		if podTemplate.ObjectMeta.Annotations == nil {
			podTemplate.ObjectMeta.Annotations = make(map[string]string)
		}
		for key, value := range k.k8sCfg.Annotations {
			podTemplate.ObjectMeta.Annotations[key] = value
		}
		log.Printf("Merge de annotations con la config: %v", k.k8sCfg.Annotations)
	}

	// 5. Imagen del contenedor (usar WorkerSpec.Image si se define, sino usar k.k8sCfg.DefaultImage, sino usar la del template)
	workerImage := k.execution.WorkerDef.Spec.Containers[0].Image
	if workerImage == "" {
		k.sendErrorMessage(outputChan, "No se definió imagen para el Pod ")
		return nil, fmt.Errorf("no se definió imagen para el Pod")

	} else {
		log.Printf("Usando imagen de WorkerSpec: %s", workerImage)
	}
	if len(podTemplate.Spec.Containers) > 0 { // Asumiendo que el primer contenedor es el worker container
		podTemplate.Spec.Containers[0].Image = workerImage
	}

	// 6. Variables de entorno (append WorkerSpec.Environment a las env vars existentes en el template)
	envVars := buildK8sEnvVars(k.execution.WorkerDef.Spec.Containers[0].Env)
	if len(podTemplate.Spec.Containers) > 0 {
		podTemplate.Spec.Containers[0].Env = append(podTemplate.Spec.Containers[0].Env, envVars...) // Append para mergear
		log.Printf("Añadiendo variables de entorno de WorkerSpec: %v", k.execution.WorkerDef.Spec.Containers[0].Env)
	}

	// **PodSpec ya está configurado desde el template y personalizado.**

	// Crear el Pod en el cluster
	namespaceToCreateIn := podTemplate.ObjectMeta.Namespace // Usar el namespace que finalmente quedó configurado en el template
	_, err = k.clientset.CoreV1().Pods(namespaceToCreateIn).Create(ctx, podTemplate, metav1.CreateOptions{})
	if err != nil {
		k.sendErrorMessage(outputChan, "Error creando Pod")
		return nil, fmt.Errorf("error creando Pod %s en namespace '%s': %v", podName, namespaceToCreateIn, err)
	}
	log.Printf("Pod %s creado en Kubernetes en namespace '%s'", podName, namespaceToCreateIn)

	// Esperar a que el Pod entre en Running
	err = waitForPodRunning(ctx, k.clientset, podName, namespaceToCreateIn)
	if err != nil {
		k.sendErrorMessage(outputChan, "Error esperando que el Pod esté en Running")
		return nil, fmt.Errorf("error esperando que el Pod %s esté en Running: %v", podName, err)
	}
	log.Printf("Pod %s está en Running en namespace '%s'", podName, namespaceToCreateIn)

	// Obtener información del Pod (IP, etc.)
	pod, err := k.clientset.CoreV1().Pods(namespaceToCreateIn).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		k.sendErrorMessage(outputChan, "Error obteniendo Pod")
		return nil, fmt.Errorf("error obteniendo Pod %s: %v", podName, err)
	}

	podIP := pod.Status.PodIP
	if podIP == "" {
		k.sendErrorMessage(outputChan, "No se encontró la IP del Pod")
		return nil, fmt.Errorf("no se encontró la IP del Pod %s", podName)
	}

	// Configurar connectionInfo
	k.connectionInfo = &model.ConnectionInfo{
		WorkerName:    podName,
		ContainerName: podName,
		Address:       fmt.Sprintf("%s:%d", podIP, pod.Spec.Containers[0].Ports[0].ContainerPort), // Primer puerto (simplificación)
		Protocol:      "http",
	}

	log.Printf("K8sWorker ConnectionInfo: Address=%s Protocol=%s", k.connectionInfo.Address, k.connectionInfo.Protocol)
	return k.connectionInfo, nil
}

// sendErrorMessage reenvía un mensaje de error al outputChan si está disponible
func (k *K8sWorker) sendErrorMessage(outputChan chan<- model.ProcessOutput, errMsg string) {
	if outputChan == nil {
		return
	}
	outputChan <- model.ProcessOutput{
		IsError:   true,
		Output:    errMsg,
		ProcessID: k.execution.ID.String(),
	}
}

// Run crea un cliente gRPC y llama StartProcess, enviando la salida a outputChan
func (k *K8sWorker) Run(ctx context.Context, t model.TaskExecution, outputChan chan<- model.ProcessOutput) error {
	grpcClient, err := k.createGRPCClient()
	if err != nil {
		return fmt.Errorf("error creando cliente gRPC: %w", err)
	}
	defer grpcClient.Close()

	cmds := k.execution.Task.Spec.Command
	if len(cmds) == 0 {
		cmds = []string{"echo", "Hola desde K8sWorker"}
	}
	envMap := k.execution.WorkerDef.Spec.Containers[0].Env
	if envMap == nil {
		envMap = []model.EnvVar{}
	}

	// Llamar al proceso remoto (StartProcess)
	if err := grpcClient.StartProcess(ctx, t.ID.String(), cmds, envMap, k.execution.WorkerDef.Spec.Containers[0].WorkingDir, outputChan); err != nil {
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

	success, msg, err := grpcClient.StopProcess(ctx, k.connectionInfo.WorkerName)
	if err != nil {
		return false, msg, fmt.Errorf("error en StopProcess (K8s): %v", err)
	}
	log.Printf("Stop K8s process success=%v, msg=%v", success, msg)

	// En caso quieras eliminar el Pod:
	// err = k.clientset.CoreV1().Pods("default").Delete(ctx, "execution-"+k.execution.Name, metav1.DeleteOptions{})
	// if err != nil { ... }

	return success, msg, nil
}

// StartMonitoring inicia la monitorización de salud
func (k *K8sWorker) StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *model.ProcessHealthStatus) error {
	grpcClient, err := k.createGRPCClient()
	if err != nil {
		return fmt.Errorf("error creando gRPC client para StartMonitoring: %w", err)
	}
	// No cerramos grpcClient aquí si necesitamos monitorizar en segundo plano
	err = grpcClient.MonitorHealth(ctx, k.connectionInfo.WorkerName, checkInterval, healthChan)
	if err != nil {
		return fmt.Errorf("error en MonitorHealth de K8s: %v", err)
	}
	return nil
}

// GetEndpoint retorna el WorkerEndpoint (Pod IP y puerto)
func (k *K8sWorker) GetEndpoint() *model.ConnectionInfo {
	return k.connectionInfo
}

// createGRPCClient construye el RPSClient con la configuración TLS o token, tal como docker_instance.go
func (k *K8sWorker) createGRPCClient() (*grpc.RPSClient, error) {
	if k.connectionInfo == nil {
		return nil, fmt.Errorf("connectionInfo no inicializado en K8sWorker")
	}
	rpcClientConfig := &grpc.RemoteProcessClientConfig{
		Address:    k.connectionInfo.Address,
		ClientCert: k.grpcConfig.ClientCertPath,
		ClientKey:  k.grpcConfig.ClientKeyPath,
		CACert:     k.grpcConfig.CACertPath,
		AuthToken:  k.token,
	}
	rpcClient, err := grpc.New(rpcClientConfig)
	if err != nil {
		return nil, fmt.Errorf("error creando remote_worker_client K8s: %v", err)
	}
	return rpcClient, nil
}

// getK8sClient crea un clientset de Kubernetes para interactuar con la API
func getK8sClient(k8sCfg resource.KubernetesResoucesPoolConfig) (*kubernetes.Clientset, error) {
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
func buildK8sEnvVars(env []model.EnvVar) []apiv1.EnvVar {
	if env == nil {
		return nil
	}
	items := make([]apiv1.EnvVar, 0, len(env))
	for _, e := range env {
		items = append(items, apiv1.EnvVar{
			Name:  e.Name,
			Value: e.Value,
		})
	}
	return items
}
