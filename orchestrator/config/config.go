package config

import (
	"log"
	"os"
	"strconv"
)

type GRPCConfig struct {
	ServerAddress string
	ClientCert    string
	ClientKey     string
	CACert        string
	JWTToken      string
}

func LoadGRPCConfig() *GRPCConfig {
	return &GRPCConfig{
		ServerAddress: getEnvironmentValue("GRPC_SERVER_ADDRESS", "localhost:50051"),
		ClientCert:    getEnvironmentValue("CLIENT_CERT_PATH", "certs/dev/client-cert.pem"),
		ClientKey:     getEnvironmentValue("CLIENT_KEY_PATH", "certs/dev/client-key.pem"),
		CACert:        getEnvironmentValue("CA_CERT_PATH", "certs/dev/ca-cert.pem"),
		JWTToken:      getEnvironmentValue("JWT_TOKEN", "default_jwt_token"),
	}
}

func GetEnv() string {
	return getEnvironmentValue("ENV", "development")
}

func GetDataSourceURL() string {
	return getEnvironmentValue("DATA_SOURCE_URL", "default_data_source_url")
}

func GetApplicationPort() int {
	portStr := getEnvironmentValue("APPLICATION_PORT", "8080")
	port, err := strconv.Atoi(portStr)

	if err != nil {
		log.Fatalf("port: %s is invalid", portStr)
	}

	return port
}

func getEnvironmentValue(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
