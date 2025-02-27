package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	// Verificar variables de entorno
	// Debug mejorado
	log.Printf("🔐 Configuración TLS:")
	log.Printf("- SERVER_CERT_PATH: %s", c.ServerCertPath)
	log.Printf("- SERVER_KEY_PATH: %s", c.ServerKeyPath)
	log.Printf("- CA_CERT_PATH: %s", c.CACertPath)

	// Obtener directorio de trabajo actual
	cwd, _ := os.Getwd()
	log.Printf("📁 Directorio de trabajo actual: %s", cwd)

	// Verificar existencia de archivos
	files := map[string]string{
		"Certificado del Servidor": c.ServerCertPath,
		"Llave del Servidor":       c.ServerKeyPath,
		"Certificado CA":           c.CACertPath,
	}

	for desc, path := range files {
		log.Printf("🔍 Verificando %s en ruta: %q", desc, path)

		// Comprobar si la ruta es absoluta
		if !filepath.IsAbs(path) {
			absPath, _ := filepath.Abs(path)
			log.Printf("⚠️ Ruta relativa detectada, versión absoluta: %s", absPath)
		}

		// Verificar acceso al archivo
		fileInfo, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				// Listar contenido del directorio padre
				dir := filepath.Dir(path)
				files, err := os.ReadDir(dir)
				fileList := "Archivos encontrados en " + dir + ":\n"
				if err != nil {
					fileList = "Error leyendo directorio: " + err.Error()
				} else {
					for _, file := range files {
						info, _ := file.Info()
						if info != nil {
							fileList += fmt.Sprintf("  - %s (%d bytes, %s)\n", file.Name(), info.Size(), info.Mode())
						} else {
							fileList += fmt.Sprintf("  - %s\n", file.Name())
						}
					}
				}
				return nil, fmt.Errorf("❌ %s no encontrado en: %s\n%s", desc, path, fileList)
			} else {
				return nil, fmt.Errorf("❌ Error accediendo a %s: %v", desc, err)
			}
		}

		// Archivo existe, mostrar detalles
		log.Printf("✅ %s encontrado: Tamaño=%d bytes, Permisos=%s",
			desc, fileInfo.Size(), fileInfo.Mode())

		// Prueba de acceso al archivo
		content, err := os.ReadFile(path)
		if err != nil {
			log.Printf("⚠️ Error intentando leer %s: %v", desc, err)
		} else {
			log.Printf("✅ %s leído correctamente: %d bytes", desc, len(content))
		}
	}

	// Cargar certificado del servidor
	certificate, err := tls.LoadX509KeyPair(c.ServerCertPath, c.ServerKeyPath)
	if err != nil {
		return nil, fmt.Errorf("❌ Error cargando certificado del servidor:\n"+
			"- Error: %v\n"+
			"- Permisos SERVER_CERT: %s\n"+
			"- Permisos SERVER_KEY: %s",
			err,
			getFilePermissions(c.ServerCertPath),
			getFilePermissions(c.ServerKeyPath))
	}

	// Cargar CA cert
	caCert, err := os.ReadFile(c.CACertPath)
	if err != nil {
		return nil, fmt.Errorf("❌ Error leyendo certificado CA:\n"+
			"- Error: %v\n"+
			"- Permisos CA_CERT: %s",
			err,
			getFilePermissions(c.CACertPath))
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		// Verificar formato del certificado
		return nil, fmt.Errorf("❌ Error añadiendo certificado CA al pool:\n"+
			"- El archivo existe pero puede no ser un certificado PEM válido\n"+
			"- Contenido del archivo (primeros 100 bytes): %s",
			string(caCert[:min(len(caCert), 100)]))
	}

	log.Println("✅ Certificados cargados correctamente")

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// Helper para obtener los permisos de un archivo
func getFilePermissions(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Sprintf("Error obteniendo permisos: %v", err)
	}
	return fmt.Sprintf("Mode: %v", info.Mode())
}

// Helper para min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
