package store

import (
	"fmt"
	"github.com/pocketbase/pocketbase"
	"log"
	"os"

	"github.com/pocketbase/pocketbase/core"
)

// Initialize prepara la base de datos y realiza las inicializaciones necesarias
func Initialize(app core.App, dbPath string) error {
	log.Println("Inicializando base de datos SQLite...")

	// Configurar la ruta de la base de datos SQLite
	if err := os.Setenv("PB_SQLITE_DB_PATH", dbPath); err != nil {
		return fmt.Errorf("error al configurar la ruta de la base de datos: %w", err)
	}

	// IMPORTANTE: PocketBase necesita inicializar su base de datos antes de realizar consultas
	// Esperamos a que PocketBase esté inicializado antes de crear colecciones

	// Para PocketBase 0.25.7, necesitamos asegurarnos de que el motor de la base de datos
	// esté completamente inicializado antes de intentar crear colecciones
	if pb, ok := app.(*pocketbase.PocketBase); ok {
		// Usar métodos específicos de PocketBase para inicializar si es necesario
		if err := pb.Bootstrap(); err != nil {
			return fmt.Errorf("error al inicializar PocketBase: %w", err)
		}
	}

	// Inicializar colecciones
	if err := InitializeCollections(app); err != nil {
		return fmt.Errorf("error al inicializar colecciones: %w", err)
	}

	// Crear superadministrador
	if err := CreateSuperAdmin(app); err != nil {
		return fmt.Errorf("error al crear superadmin: %w", err)
	}

	log.Printf("Base de datos inicializada correctamente en: %s", dbPath)
	return nil
}
