package config

import (
	"log"
	"os"
)

func GetEnv() string {
	return getEnvironmentValue("ENV")
}

func GetDataSourceURL() string {
	return getEnvironmentValue("DATA_SOURCE_URL")
}

func GetCertFile() string {
	return getEnvironmentValue("CERT_FILE")
}

func GetKeyFile() string {
	return getEnvironmentValue("KEY_FILE")
}

func GetApplicationPort() string {
	return getEnvironmentValue("APPLICATION_PORT")
}

func getEnvironmentValue(key string) string {
	if os.Getenv(key) == "" {
		log.Fatalf("%s environment variable is missing.", key)
	}

	return os.Getenv(key)
}
