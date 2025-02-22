package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config general de la aplicación
type Config struct {
	Env                string
	Port               int
	WorkerName         string
	MaxConcurrentTasks int
	StorageType        string
	GRPC               GRPCConfig
	Docker             DockerConfig

	// Nueva sección para distintos proveedores
	Providers ProvidersConfig
}

// Configuración general de gRPC
type GRPCConfig struct {
	// Certificados y claves
	ServerCertPath string // Para remote-process
	ServerKeyPath  string // Para remote-process
	ClientCertPath string // Para worker
	ClientKeyPath  string // Para worker
	CACertPath     string // Compartido

	// Autenticación
	JWTSecret string
	JWTToken  string

	// Entorno
	Environment string
}

// Config que engloba distintos proveedores: Docker, K8s, VMs...
type ProvidersConfig struct {
	Docker      DockerConfig
	Kubernetes  K8sConfig
	VirtualMach VMConfig // Ejemplo, máquinas virtuales
}

// Config específica para Docker
type DockerConfig struct {
	Host            string
	DefaultImage    string
	CertsVolumePath string // Ruta al directorio de certificados (puede ser absoluta o relativa)
	CertsMountPath  string // Punto de montaje en el contenedor (siempre /certs)
	HealthCheck     HealthCheck
	NetworkName     string        // Para asegurar que los contenedores están en la misma red
	StopDelay       time.Duration // Tiempo de espera antes de parar el worker
}

// Config específica de Kubernetes
type K8sConfig struct {
	Namespace    string
	KubeConfig   string            // Ruta a kubeconfig si estás fuera del cluster
	InCluster    bool              // Indica si ejecuta dentro del cluster
	Labels       map[string]string // Labels por defecto en Pods
	Annotations  map[string]string // Anotaciones disponibles
	DefaultImage string
}

// Config de máquinas virtuales (solo un ejemplo genérico)
type VMConfig struct {
	HypervisorURL  string // e.g., "qemu:///system", "vcenter.local"
	DefaultNetwork string
	// etc.
}

type HealthCheck struct {
	Test     []string
	Interval time.Duration
	Timeout  time.Duration
	Retries  int
}

// Cargas la configuración unificada
func Load() Config {
	// Obtener el directorio actual para la ruta por defecto
	pwd, err := os.Getwd()
	defaultCertsPath := "./certs/dev"
	if err == nil {
		defaultCertsPath = filepath.Join(pwd, "certs", "dev")
	}

	return Config{
		Env:                getEnv("ENV", "development"),
		Port:               getIntEnv("HTTP_PORT", 8080),
		WorkerName:         getEnv("WORKER_NAME", "default-worker"),
		MaxConcurrentTasks: getIntEnv("MAX_CONCURRENT_TASKS", 3),
		StorageType:        getEnv("STORAGE_TYPE", "memory"),

		GRPC: GRPCConfig{
			// Certificados
			ServerCertPath: getEnv("SERVER_CERT_PATH", "/certs/remote_process-cert.pem"),
			ServerKeyPath:  getEnv("SERVER_KEY_PATH", "/certs/remote_process-key.pem"),
			ClientCertPath: getEnv("CLIENT_CERT_PATH", "/certs/worker-cert.pem"),
			ClientKeyPath:  getEnv("CLIENT_KEY_PATH", "/certs/worker-key.pem"),
			CACertPath:     getEnv("CA_CERT_PATH", "/certs/ca-cert.pem"),

			// Autenticación
			JWTSecret: getEnv("JWT_SECRET", ""),
			JWTToken:  getEnv("JWT_TOKEN", ""),

			// Entorno
			Environment: getEnv("ENV", "development"),
		},

		Providers: ProvidersConfig{
			Docker: DockerConfig{
				Host:            getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
				DefaultImage:    getEnv("DOCKER_DEFAULT_IMAGE", "posts_mpv-remote-process:latest"),
				CertsVolumePath: getEnv("DOCKER_CERTS_PATH", defaultCertsPath),
				CertsMountPath:  "/certs",
				NetworkName:     getEnv("DOCKER_NETWORK", "devops-platform_default"),
				StopDelay:       getDurationEnv("DOCKER_STOP_DELAY", 10*time.Second),
				HealthCheck: HealthCheck{
					Test:     []string{"CMD", "/app/grpc_health_check.sh"},
					Interval: getDurationEnv("DOCKER_HEALTHCHECK_INTERVAL", 3*time.Second),
					Timeout:  getDurationEnv("DOCKER_HEALTHCHECK_TIMEOUT", 5*time.Second),
					Retries:  getIntEnv("DOCKER_HEALTHCHECK_RETRIES", 2),
				},
			},
			Kubernetes: K8sConfig{
				Namespace:   getEnv("K8S_NAMESPACE", "default"),
				KubeConfig:  getEnv("K8S_CONFIG_PATH", ""),
				InCluster:   getBoolEnv("K8S_IN_CLUSTER", true),
				Labels:      map[string]string{"app": "grpc-worker"},
				Annotations: map[string]string{},
			},
			VirtualMach: VMConfig{
				HypervisorURL:  getEnv("VM_HYPERVISOR_URL", ""),
				DefaultNetwork: getEnv("VM_DEF_NETWORK", ""),
			},
		},
	}
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getIntEnv(key string, defaultValue int) int {
	strValue := getEnv(key, "")
	if strValue == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(strValue)
	if err != nil {
		log.Fatalf("Invalid value for %s: %v", key, err)
	}
	return value
}

func getBoolEnv(key string, defaultValue bool) bool {
	strValue := getEnv(key, "")
	if strValue == "" {
		return defaultValue
	}
	return strValue == "true" || strValue == "1"
}

// Helper para parsear duraciones desde variables de entorno
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}
