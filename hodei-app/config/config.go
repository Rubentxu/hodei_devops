package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	idGenerator "dev.rubentxu.hodei-devops/hodei-app/internal/adapters/outgoing/repository"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/application/scheduler"
	"dev.rubentxu.hodei-devops/hodei-app/internal/domain/ports"
	"log"
)

// Config general de la aplicación
type Config struct {
	Environment               string
	Port                      int
	MaxConcurrentTasks        int
	DefaultDockerResourcePool bool
	MongoDBURI                string
	MongoDBName               string
	IdGenerator               ports.IDGenerator
	ServerCertPath            string
	ServerKeyPath             string
	ClientCertPath            string
	ClientKeyPath             string
	CACertPath                string
	AccessSecret              string
	RefreshSecret             string
	AccessExpiry              time.Duration
	RefreshExpiry             time.Duration
	PasswordHasherDefaultCost int
	Scheduler                 ports.Scheduler
}

func Load() Config {
	mongoDBURI := fmt.Sprintf("mongodb://%s:%s@%s:%s/%s?authSource=%s",
		getEnv("MONGODB_USER", "user"),
		getEnv("MONGODB_PASSWORD", "user"),
		getEnv("MONGODB_HOST", "localhost"),
		getEnv("MONGODB_PORT", "27017"),
		getEnv("MONGODB_DATABASE", "hodeidb"),
		getEnv("MONGODB_DATABASE", "hodeidb"))

	return Config{
		Environment:               getEnv("ENVIROMENT", "development"),
		Port:                      getIntEnv("HTTP_PORT", 8080),
		MaxConcurrentTasks:        getIntEnv("MAX_CONCURRENT_TASKS", 3),
		DefaultDockerResourcePool: getBoolEnv("DEFAULT_DOCKER_POOL", false),
		MongoDBURI:                mongoDBURI,
		IdGenerator:               createIDGenerator(getEnv("ID_GENERATOR_TYPE", "bson")),
		ServerCertPath:            getEnv("SERVER_CERT_PATH", "/certs/remote_worker-cert.pem"),
		ServerKeyPath:             getEnv("SERVER_KEY_PATH", "/certs/remote_worker-key.pem"),
		ClientCertPath:            getEnv("CLIENT_CERT_PATH", "/certs/worker-client-cert.pem"),
		ClientKeyPath:             getEnv("CLIENT_KEY_PATH", "/certs/worker-client-key.pem"),
		CACertPath:                getEnv("CA_CERT_PATH", "/certs/ca-cert.pem"),
		AccessSecret:              getEnv("JWT_ACCESS_SECRET", "default_access_secret"),
		RefreshSecret:             getEnv("JWT_REFRESH_SECRET", "default_refresh_secret"),
		AccessExpiry:              getDurationFromEnv("JWT_ACCESS_EXPIRY", 15*time.Minute),
		RefreshExpiry:             getDurationFromEnv("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		PasswordHasherDefaultCost: getIntEnv("BCRYPT_WORK_FACTOR", 12),
		Scheduler:                 ResolveScheduler(getEnv("SCHEDULER_TYPE", "greedy")),
	}
}

func ResolveScheduler(schedulerType string) ports.Scheduler {
	switch strings.ToLower(schedulerType) {
	case "epvm":
		return scheduler.NewEpvm()
	case "greedy":
		return scheduler.NewGreedy()
	case "roundrobin":
		return scheduler.NewRoundRobin()
	default:
		log.Printf("Tipo de scheduler no reconocido: %s, usando greedy por defecto", schedulerType)
		return scheduler.NewGreedy()
	}
}

func createIDGenerator(generatorType string) ports.IDGenerator {
	return idGenerator.NewIDGenerator(generatorType)
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

func getDurationFromEnv(key string, defaultValue time.Duration) time.Duration {
	if durationStr, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(durationStr); err == nil {
			return duration
		}
	}
	return defaultValue
}
