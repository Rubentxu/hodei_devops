package store

import (
	"fmt"
	"log"

	"github.com/pocketbase/pocketbase/core"
)

func CreateSuperAdmin(app core.App) error {
	log.Println("Verificando/creando superusuario...")

	// Verificar si ya existe un superusuario
	superusers, err := app.FindCollectionByNameOrId(core.CollectionNameSuperusers)
	if err != nil {
		// Si la colección no existe todavía, intenta crearla primero
		// Esto puede ocurrir en la primera ejecución
		log.Printf("No se encontró la colección de superusuarios: %v", err)
		log.Println("PocketBase debería crearla automáticamente durante el inicio")
		return nil
	}

	// Datos del superadmin
	email := "rubentxu74@gmail.com"
	password := "cabrera"

	// Verificar si ya existe un usuario con este email
	existingAdmin, _ := app.FindAuthRecordByEmail(core.CollectionNameSuperusers, email)
	if existingAdmin != nil {
		log.Printf("El superusuario %s ya existe, omitiendo creación", email)
		return nil
	}

	// Crear el registro de superadmin
	record := core.NewRecord(superusers)
	record.Set("email", email)
	record.Set("password", password)
	record.Set("verified", true) // Marcar como verificado para evitar el proceso de verificación
	//record.Set("tokenKey", types.SecureString(15)) // Generar una clave de token segura

	if err := app.SaveNoValidate(record); err != nil {
		return fmt.Errorf("error al crear superusuario: %w", err)
	}

	log.Println("╔════════════════════════════════════════════╗")
	log.Println("║  Superusuario creado exitosamente          ║")
	log.Println("║────────────────────────────────────────────║")
	log.Printf("║  Email: %-35s ║\n", email)
	log.Printf("║  Contraseña: %-30s ║\n", password)
	log.Println("╚════════════════════════════════════════════╝")

	return nil
}
