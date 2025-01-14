package config

import (
	"log"
	"os"
)

func GetEnv() string {
	return getEnvironmentValue("ENV")
}

func GetApplicationPort() string {
	return getEnvironmentValue("APPLICATION_PORT")
}

func GetJWTSecret() string {
	return getEnvironmentValue("JWT_SECRET")
}

func getEnvironmentValue(key string) string {
	if os.Getenv(key) == "" {
		log.Fatalf("%s environment variable is missing.", key)
	}

	return os.Getenv(key)
}
