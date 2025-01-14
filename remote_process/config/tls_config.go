package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

type TLSConfig struct {
	// Rutas a los certificados
	ServerCertPath string
	ServerKeyPath  string
	CACertPath     string
}

// LoadTLSConfig carga la configuración TLS desde variables de entorno
func LoadTLSConfig() (*TLSConfig, error) {
	return &TLSConfig{
		ServerCertPath: getEnvironmentValue("SERVER_CERT_PATH"),
		ServerKeyPath:  getEnvironmentValue("SERVER_KEY_PATH"),
		CACertPath:     getEnvironmentValue("CA_CERT_PATH"),
	}, nil
}

// ConfigureServerTLS configura el TLS para el servidor gRPC
func (c *TLSConfig) ConfigureServerTLS() (*tls.Config, error) {
	// Cargar certificado del servidor
	certificate, err := tls.LoadX509KeyPair(c.ServerCertPath, c.ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %v", err)
	}

	// Cargar CA cert
	caCert, err := os.ReadFile(c.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	// Configurar TLS
	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}
