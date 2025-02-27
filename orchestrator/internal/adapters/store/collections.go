package store

import (
	"fmt"
	"log"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// InitializeCollections crea todas las colecciones necesarias para la aplicación
func InitializeCollections(app core.App) error {
	log.Println("Creando colecciones en la base de datos...")

	// Inicializar colección de templates de worker
	if err := createWorkerTemplatesCollection(app); err != nil {
		// Si el error no es crítico, continuamos
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		log.Printf("Advertencia: %v", err)
	}

	// Inicializar colección de configuraciones de resource pool
	if err := createResourcePoolConfigsCollection(app); err != nil {
		// Si el error no es crítico, continuamos
		if !strings.Contains(err.Error(), "already exists") {
			return err
		}
		log.Printf("Advertencia: %v", err)
	}

	log.Println("Todas las colecciones han sido procesadas")
	return nil
}

// createWorkerTemplatesCollection crea la colección para templates de worker
func createWorkerTemplatesCollection(app core.App) error {
	collectionName := "worker_templates"

	log.Printf("Verificando colección %s...", collectionName)

	// Verificar si la colección ya existe - primero intentamos con un manejo de error más robusto
	_, err := app.FindCollectionByNameOrId(collectionName)
	if err == nil {
		log.Printf("La colección %s ya existe", collectionName)
		return nil
	}

	// Solo continuar si el error indica que la colección no existe
	if !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error inesperado al buscar colección %s: %w", collectionName, err)
	}

	log.Printf("Creando colección %s...", collectionName)
	collection := core.NewBaseCollection(collectionName)
	collection.Fields.Add(
		&core.TextField{
			Name:     "template_name",
			Required: true,
			Max:      100,
		},
		&core.JSONField{
			Name:        "template_data",
			Required:    true,
			Presentable: true,
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Añadir índice para template_name
	collection.AddIndex("idx_template_name", true, "template_name", "")

	if err := app.SaveNoValidate(collection); err != nil {
		return fmt.Errorf("error al crear colección %s: %w", collectionName, err)
	}

	log.Printf("Colección %s creada exitosamente", collectionName)
	return nil
}

// createResourcePoolConfigsCollection crea la colección para configuraciones de resource pool
func createResourcePoolConfigsCollection(app core.App) error {
	collectionName := "resource_pool_configs"

	log.Printf("Verificando colección %s...", collectionName)

	// Verificar si la colección ya existe - con manejo de error más robusto
	_, err := app.FindCollectionByNameOrId(collectionName)
	if err == nil {
		log.Printf("La colección %s ya existe", collectionName)
		return nil
	}

	// Solo continuar si el error indica que la colección no existe
	if !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("error inesperado al buscar colección %s: %w", collectionName, err)
	}

	log.Printf("Creando colección %s...", collectionName)
	collection := core.NewBaseCollection(collectionName)
	collection.Fields.Add(
		&core.TextField{
			Name:     "config_name",
			Required: true,
			Max:      100,
		},
		&core.JSONField{
			Name:        "config_data",
			Required:    true,
			Presentable: true,
		},
		&core.AutodateField{
			Name:     "created",
			OnCreate: true,
		},
		&core.AutodateField{
			Name:     "updated",
			OnCreate: true,
			OnUpdate: true,
		},
	)

	// Añadir índice para config_name
	collection.AddIndex("idx_config_name", true, "config_name", "")

	if err := app.SaveNoValidate(collection); err != nil {
		return fmt.Errorf("error al crear colección %s: %w", collectionName, err)
	}

	log.Printf("Colección %s creada exitosamente", collectionName)
	return nil
}
