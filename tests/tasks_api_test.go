package integration

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	orchestratorPort = "8090/tcp"
)

var (
	// Variables globales para compartir entre todos los tests
	orchestratorContainerName = "hodei-orchestrator"
	orchestratorContainer     testcontainers.Container // Única instancia para todos los tests
	orchestratorNetworkName   string
	testNetwork               *testcontainers.DockerNetwork

	// Para asegurar que el contenedor está listo antes de ejecutar tests
	containerReady = make(chan struct{})
	containerError = make(chan error, 1)
)

// WSMessage define la estructura de los mensajes WebSocket
type WSMessage struct {
	Action  string      `json:"action"`
	Payload interface{} `json:"payload"`
}

// createTaskRequest define la estructura para crear una tarea
type createTaskRequest struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Command      []string          `json:"command,omitempty"`
	Env          map[string]string `json:"env,omitempty"`
	WorkingDir   string            `json:"working_dir,omitempty"`
	InstanceType string            `json:"instance_type,omitempty"`
}

// TaskTestCase define la estructura de un caso de prueba de tarea
type TaskTestCase struct {
	Name         string
	Image        string
	Command      []string
	Env          map[string]string
	WorkingDir   string
	InstanceType string
	Duration     time.Duration
}

// forceRemoveContainer usa exec.Command para forzar la eliminación del contenedor
func forceRemoveContainer(containerName string) error {
	log.Printf("🗑️ Forzando eliminación del contenedor %s usando CLI docker", containerName)

	// Primero verificamos si el contenedor existe
	checkCmd := exec.Command("docker", "ps", "-a", "--filter", fmt.Sprintf("name=%s", containerName), "--format", "{{.Names}}")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("error verificando existencia del contenedor: %w", err)
	}

	// Si no hay salida, el contenedor no existe
	if len(strings.TrimSpace(string(output))) == 0 {
		log.Printf("ℹ️ No se encontró ningún contenedor con el nombre %s", containerName)
		return nil
	}

	log.Printf("🔍 Contenedor encontrado: %s", strings.TrimSpace(string(output)))

	// Intentar detener el contenedor primero (ignoramos errores)
	stopCmd := exec.Command("docker", "stop", "--time=10", containerName)
	_ = stopCmd.Run() // Ignoramos errores, puede que ya esté detenido

	// Ahora forzamos la eliminación
	removeCmd := exec.Command("docker", "rm", "-f", containerName)
	if removeOutput, err := removeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error eliminando contenedor %s: %w\nOutput: %s",
			containerName, err, string(removeOutput))
	}

	log.Printf("✅ Contenedor %s eliminado correctamente", containerName)
	return nil
}

// forceRemoveNetwork usa exec.Command para forzar la eliminación de la red
func forceRemoveNetwork(networkName string) error {
	if networkName == "" {
		return nil
	}

	log.Printf("🗑️ Forzando eliminación de la red %s usando CLI docker", networkName)

	// Verificamos si la red existe
	checkCmd := exec.Command("docker", "network", "ls", "--filter", fmt.Sprintf("name=%s", networkName), "--format", "{{.Name}}")
	output, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("error verificando existencia de la red: %w", err)
	}

	// Si no hay salida, la red no existe
	if len(strings.TrimSpace(string(output))) == 0 {
		log.Printf("ℹ️ No se encontró ninguna red con el nombre %s", networkName)
		return nil
	}

	log.Printf("🔍 Red encontrada: %s", strings.TrimSpace(string(output)))

	// Forzamos la eliminación
	removeCmd := exec.Command("docker", "network", "rm", networkName)
	if removeOutput, err := removeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("error eliminando red %s: %w\nOutput: %s",
			networkName, err, string(removeOutput))
	}

	log.Printf("✅ Red %s eliminada correctamente", networkName)
	return nil
}

func TestMain(m *testing.M) {
	// Contexto principal para todo el ciclo de vida de los tests
	ctx := context.Background()
	var exitCode int

	// 1. Limpieza previa de recursos que podrían haber quedado de ejecuciones anteriores
	log.Println("🧹 Realizando limpieza previa de recursos...")

	// Primero limpiamos el contenedor usando fuerza bruta
	if err := forceRemoveContainer(orchestratorContainerName); err != nil {
		fmt.Printf("❌ Error en limpieza previa del contenedor: %v\n", err)
		os.Exit(1)
	}

	// Limpiar cualquier red existente con el mismo nombre (si se especificó)
	if networkName := os.Getenv("TEST_NETWORK_NAME"); networkName != "" {
		if err := forceRemoveNetwork(networkName); err != nil {
			fmt.Printf("❌ Error en limpieza previa de la red: %v\n", err)
			os.Exit(1)
		}
	}

	// 2. Inicializar la red y el contenedor
	log.Println("🚀 Inicializando entorno de pruebas...")

	// Crear nombre de red único para este test
	networkNameSuffix := fmt.Sprintf("test-orch-%d", time.Now().Unix())
	if envNetName := os.Getenv("TEST_NETWORK_NAME"); envNetName != "" {
		networkNameSuffix = envNetName
	}

	// Crear red Docker
	log.Printf("🌐 Creando red Docker: %s", networkNameSuffix)
	var err error
	testNetwork, err = network.New(ctx,

		network.WithAttachable(),
		network.WithLabels(map[string]string{
			"test": "orchestrator-test",
		}),
	)
	if err != nil {
		fmt.Printf("❌ Error creando red: %v\n", err)
		os.Exit(1)
	}
	orchestratorNetworkName = testNetwork.Name
	log.Printf("✅ Red Docker creada: %s", orchestratorNetworkName)

	// Iniciar el contenedor
	log.Println("🚀 Iniciando contenedor orchestrator...")
	orchestratorContainer, err = startOrchestrator(ctx, orchestratorNetworkName)
	if err != nil {
		fmt.Printf("❌ Error iniciando orchestrator: %v\n", err)
		// Limpiar la red si hay error
		if testNetwork != nil {
			_ = testNetwork.Remove(ctx)
		}
		os.Exit(1)
	}
	log.Println("✅ Contenedor orchestrator iniciado correctamente")

	// 3. Ejecutar todos los tests con la misma instancia de contenedor
	log.Println("🧪 Ejecutando tests...")
	exitCode = m.Run()

	// 4. Limpieza al finalizar todos los tests
	log.Println("🧹 Limpiando recursos...")

	// Guardar logs antes de terminar
	if orchestratorContainer != nil {
		err = saveContainerLogs(ctx, orchestratorContainer, "../bin/orchestrator_logs.txt")
		if err != nil {
			fmt.Printf("⚠️ Error al guardar logs del contenedor: %v\n", err)
		}

		// Terminar el contenedor
		log.Println("⏹️ Terminando contenedor orchestrator...")
		if err := orchestratorContainer.Terminate(ctx); err != nil {
			fmt.Printf("⚠️ Error terminando contenedor: %v\n", err)
			// Si falla la terminación normal, intentar con fuerza bruta
			if forceErr := forceRemoveContainer(orchestratorContainerName); forceErr != nil {
				fmt.Printf("⚠️ Error en limpieza forzada del contenedor: %v\n", forceErr)
			}
		}
	}

	// Eliminar la red
	if testNetwork != nil {
		log.Println("🗑️ Eliminando red de pruebas...")
		if err := testNetwork.Remove(ctx); err != nil {
			fmt.Printf("⚠️ Error eliminando red: %v\n", err)
			// Si falla, intentar con fuerza bruta
			if forceErr := forceRemoveNetwork(orchestratorNetworkName); forceErr != nil {
				fmt.Printf("⚠️ Error en limpieza forzada de la red: %v\n", forceErr)
			}
		}
	}

	os.Exit(exitCode)
}

func startOrchestrator(ctx context.Context, networkName string) (testcontainers.Container, error) {
	// Doble verificación: forzar eliminación de cualquier contenedor con el mismo nombre
	if err := forceRemoveContainer(orchestratorContainerName); err != nil {
		return nil, fmt.Errorf("no se pudo eliminar el contenedor existente: %w", err)
	}

	// Configurar rutas necesarias
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo directorio actual: %w", err)
	}

	certPath := filepath.Join(pwd, "../hodei-chart/certs/dev")
	testDataPath := filepath.Join(pwd, "../test_pb_data")

	// Verificar que los directorios existen
	for _, path := range []string{certPath, testDataPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("el directorio %s no existe", path)
		}
	}

	// Configurar build del Dockerfile
	buildContext := filepath.Join(pwd, "..")
	dockerfile := "build/orchestrator/Dockerfile"

	// Obtener la imagen del worker a utilizar
	workerImage := getWorkerImage()
	log.Printf("📦 Usando imagen de worker: %s", workerImage)

	req := testcontainers.ContainerRequest{
		Name:         orchestratorContainerName,
		Image:        "", // Se construirá desde Dockerfile
		ExposedPorts: []string{orchestratorPort},
		Networks:     []string{networkName},
		Env: map[string]string{
			"CLIENT_CERT_PATH": "/certs/worker-client-cert.pem",
			"CLIENT_KEY_PATH":  "/certs/worker-client-key.pem",
			"CA_CERT_PATH":     "/certs/ca-cert.pem",
			"JWT_TOKEN":        os.Getenv("JWT_TOKEN"),
			"JWT_SECRET":       os.Getenv("JWT_SECRET"),
			"WORKER_IMAGE":     workerImage,
			"DOCKER_NETWORK":   networkName,
			"PB_SKIP_INIT":     "true",
			"PB_ADDR":          "0.0.0.0:8090",
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(certPath, "/certs"),
			testcontainers.BindMount(testDataPath, "/data"),
			testcontainers.BindMount("/var/run/docker.sock", "/var/run/docker.sock"),
		),
		WaitingFor: wait.ForHTTP("/health").
			WithPort("8090").
			WithStartupTimeout(60 * time.Second).
			WithPollInterval(1 * time.Second),
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.Privileged = true
			hc.NetworkMode = container.NetworkMode(networkName)
		},
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    buildContext,
			Dockerfile: dockerfile,
			KeepImage:  true, // Mantener imagen después del test
		},
	}

	log.Printf("🏗️ Construyendo e iniciando contenedor orchestrator: %s", orchestratorContainerName)
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("error creando contenedor: %w", err)
	}

	// Verificar que el contenedor está en funcionamiento obteniendo su IP
	ip, err := container.ContainerIP(ctx)
	if err != nil {
		return nil, fmt.Errorf("error obteniendo IP del contenedor: %w", err)
	}
	log.Printf("✅ Contenedor orchestrator iniciado correctamente en IP: %s", ip)

	return container, nil
}

// getWSBaseURL obtiene la URL base para conexiones WebSocket
func getWSBaseURL(t *testing.T, ctx context.Context) string {
	require.NotNil(t, orchestratorContainer, "El contenedor orchestrator no está inicializado")

	host, err := orchestratorContainer.Host(ctx)
	require.NoError(t, err)

	port, err := orchestratorContainer.MappedPort(ctx, "8090")
	require.NoError(t, err)

	// Se agrega '/ws' para apuntar al endpoint correcto
	return fmt.Sprintf("ws://%s:%s/ws", host, port.Port())
}

// saveContainerLogs obtiene los logs del contenedor y los escribe en el fichero especificado.
func saveContainerLogs(ctx context.Context, container testcontainers.Container, logFilePath string) error {
	if container == nil {
		return fmt.Errorf("contenedor no inicializado")
	}

	// Obtener el stream de logs del contenedor.
	logs, err := container.Logs(ctx)
	if err != nil {
		return err
	}
	defer logs.Close()

	// Asegurarse de que el directorio existe
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return fmt.Errorf("error creando directorio para logs: %w", err)
	}

	// Crear el fichero en el directorio bin.
	file, err := os.Create(logFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Copia los logs al fichero.
	_, err = io.Copy(file, logs)
	return err
}

// getWorkerImage obtiene la imagen del worker a utilizar desde variables de entorno
func getWorkerImage() string {
	if image := os.Getenv("WORKER_IMAGE"); image != "" {
		return image
	}
	return "hodei/remote-process-worker:latest" // Imagen predeterminada del Makefile
}

// TestSetup se ejecuta antes de los tests para verificar requisitos
func TestSetup(t *testing.T) {
	// Verificar que la imagen del worker existe
	image := getWorkerImage()
	cmd := exec.Command("docker", "image", "inspect", image)
	if err := cmd.Run(); err != nil {
		t.Fatalf("La imagen %s no existe localmente. Por favor, construye la imagen primero usando 'make build-remote-process'", image)
	}

	// Verificar que el contenedor orquestrador está en funcionamiento
	require.NotNil(t, orchestratorContainer, "El contenedor orquestrador no está inicializado")

	ctx := context.Background()
	state, err := orchestratorContainer.State(ctx)
	require.NoError(t, err, "Error obteniendo estado del contenedor")
	require.True(t, state.Running, "El contenedor orquestrador no está en ejecución")

	t.Log("✅ Verificaciones iniciales completadas correctamente")
}

func TestCreateTaskViaWS(t *testing.T) {
	ctx := context.Background()

	// Usamos la función para obtener la URL base desde variables de entorno
	wsURL := getWSBaseURL(t, ctx)
	t.Logf("🔌 Connecting to WebSocket URL: %s", wsURL)

	// Opcional: header con token
	header := http.Header{}
	if token := os.Getenv("JWT_TOKEN"); token != "" {
		header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	c, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
	if err != nil {
		t.Fatalf("❌ Error al abrir WebSocket: %v", err)
	}
	defer c.Close()
	defer resp.Body.Close()

	// Preparamos el payload de creación - usando la imagen correcta de worker
	taskReq := createTaskRequest{
		Name:         "test-task-WS",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "find /etc -type f -o -type d | head -5"},
		Env:          map[string]string{"EXAMPLE_KEY": "example_value"},
		WorkingDir:   "/etc",
		InstanceType: "docker",
	}

	// Enviamos mensaje con "Action" = "create_task"
	msg := WSMessage{
		Action:  "create_task",
		Payload: taskReq,
	}
	if err := c.WriteJSON(msg); err != nil {
		t.Fatalf("❌ Error al enviar JSON por WebSocket: %v", err)
	}

	// Leemos los mensajes devueltos hasta que se cierre la conexión
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var response WSMessage
			if err := c.ReadJSON(&response); err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					t.Log("✅ Conexión cerrada normalmente")
					return
				}
				t.Logf("⚠️ Error leyendo mensaje: %v", err)
				return
			}
			t.Logf("📨 Mensaje recibido: %+v", response)

			// Si encontramos un estado FINISHED, podemos terminar el test
			if payload, ok := response.Payload.(map[string]interface{}); ok {
				if status, ok := payload["status"].(string); ok && status == "FINISHED" {
					t.Log("✅ Tarea completada correctamente")
					return
				}
			}
		}
	}()

	// Esperar con timeout
	select {
	case <-done:
		// Todo correcto, el test termina normalmente
	case <-time.After(30 * time.Second):
		t.Fatal("❌ Timeout esperando respuesta del WebSocket")
	}
}

var TaskTestCases = []TaskTestCase{
	{
		Name:         "list-files-task",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "find /etc -type f | head -n 5"},
		Env:          map[string]string{"SEARCH_PATH": "/etc"},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "system-info-task",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "uname -a && cat /etc/os-release"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "memory-info-task",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "free -m || echo 'free command not available'"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "network-test-task",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "ip addr || ifconfig"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "process-list-task",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "ps aux | head -n 5 || ps | head -n 5"},
		Env:          map[string]string{},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
	{
		Name:         "custom-env-task",
		Image:        getWorkerImage(),
		Command:      []string{"sh", "-c", "env | sort"},
		Env:          map[string]string{"CUSTOM_VAR1": "value1", "CUSTOM_VAR2": "value2"},
		WorkingDir:   "/",
		InstanceType: "docker",
		Duration:     10 * time.Second,
	},
}

func TestParallelTasks(t *testing.T) {
	// Número máximo de tareas concurrentes
	maxConcurrent := 3

	// Crear un canal para limitar la concurrencia
	semaphore := make(chan struct{}, maxConcurrent)

	// Crear un WaitGroup para esperar a que todas las tareas terminen
	var wg sync.WaitGroup

	// Verificar que el orquestrador está en ejecución antes de iniciar tests paralelos
	ctx := context.Background()
	state, err := orchestratorContainer.State(ctx)
	require.NoError(t, err, "Error verificando estado del contenedor orquestrador")
	require.True(t, state.Running, "El contenedor orquestrador debe estar en ejecución para los tests paralelos")

	for _, tc := range TaskTestCases {
		tc := tc // Crear una nueva variable para cada iteración
		wg.Add(1)

		go func() {
			// Adquirir un slot del semáforo
			semaphore <- struct{}{}
			defer func() {
				// Liberar el slot del semáforo
				<-semaphore
				wg.Done()
			}()

			t.Run(tc.Name, func(t *testing.T) {
				t.Parallel() // Marcar el subtest como paralelo

				// Contexto con timeout de 90 segundos
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()

				// Usamos la función para obtener la URL base desde variables de entorno
				wsURL := getWSBaseURL(t, ctx)
				t.Logf("🔌 Connecting to WebSocket URL: %s for task: %s", wsURL, tc.Name)

				header := http.Header{}
				if token := os.Getenv("JWT_TOKEN"); token != "" {
					header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
				}

				c, resp, err := websocket.DefaultDialer.DialContext(ctx, wsURL, header)
				if err != nil {
					t.Fatalf("❌ [%s] Error al abrir WebSocket: %v", tc.Name, err)
				}

				// Cerramos la conexión después de completar el test
				defer c.Close()
				defer resp.Body.Close()

				// Crear y enviar la tarea
				taskReq := createTaskRequest{
					Name:         tc.Name,
					Image:        tc.Image,
					Command:      tc.Command,
					Env:          tc.Env,
					WorkingDir:   tc.WorkingDir,
					InstanceType: tc.InstanceType,
				}

				msg := WSMessage{
					Action:  "create_task",
					Payload: taskReq,
				}

				if err := c.WriteJSON(msg); err != nil {
					t.Fatalf("❌ [%s] Error al enviar JSON por WebSocket: %v", tc.Name, err)
				}

				// Canal para señalizar que la tarea ha terminado
				taskFinished := make(chan struct{})
				taskError := make(chan error, 1)

				// Leer respuestas
				go func() {
					defer close(taskFinished)

					for {
						var response WSMessage
						err := c.ReadJSON(&response)
						if err != nil {
							if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) {
								t.Logf("✅ [%s] Conexión cerrada por el servidor", tc.Name)
								return
							}
							select {
							case taskError <- fmt.Errorf("❌ [%s] Error leyendo mensaje: %v", tc.Name, err):
							default:
							}
							return
						}

						t.Logf("📨 [%s] Mensaje recibido: %+v", tc.Name, response)

						// Verificar si la tarea ha terminado
						if payload, ok := response.Payload.(map[string]interface{}); ok {
							if status, ok := payload["status"].(string); ok && status == "FINISHED" {
								t.Logf("✅ [%s] Tarea completada exitosamente", tc.Name)
								return
							}
						}
					}
				}()

				// Esperar a que termine la tarea o se agote el tiempo
				select {
				case <-ctx.Done():
					if ctx.Err() == context.DeadlineExceeded {
						t.Errorf("⏱️ [%s] Timeout alcanzado después de 90 segundos", tc.Name)
					}
				case err := <-taskError:
					t.Error(err)
				case <-taskFinished:
					t.Logf("✅ [%s] Tarea completada correctamente", tc.Name)
				}
			})
		}()
	}

	// Esperar a que todas las tareas terminen
	wg.Wait()
}
