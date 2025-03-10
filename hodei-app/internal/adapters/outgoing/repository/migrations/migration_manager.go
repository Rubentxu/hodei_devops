package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// MigrationManager gestiona las migraciones de la base de datos PostgreSQL
type MigrationManager struct {
	db         *sql.DB
	scriptPath string
}

// NewMigrationManager crea una nueva instancia de MigrationManager
func NewMigrationManager(db *sql.DB, scriptPath string) *MigrationManager {
	return &MigrationManager{
		db:         db,
		scriptPath: scriptPath,
	}
}

// InitializePostgres inicializa la conexión a PostgreSQL y ejecuta las migraciones
func InitializePostgres(connStr string, scriptPath string) (*sql.DB, error) {
	log.Println("Inicializando conexión a PostgreSQL...")

	// Abrir conexión a PostgreSQL
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error al conectar a PostgreSQL: %w", err)
	}

	// Verificar conexión
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al verificar conexión PostgreSQL: %w", err)
	}

	// Configurar pool de conexiones
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Crear el gestor de migraciones
	migrationManager := NewMigrationManager(db, scriptPath)

	// Inicializar la tabla de migraciones
	if err := migrationManager.InitMigrationTable(); err != nil {
		db.Close()
		return nil, err
	}

	// Ejecutar todas las migraciones pendientes
	if err := migrationManager.RunMigrations(); err != nil {
		db.Close()
		return nil, err
	}

	log.Println("Base de datos PostgreSQL inicializada correctamente")
	return db, nil
}

// InitMigrationTable crea la tabla para registrar las migraciones aplicadas
func (m *MigrationManager) InitMigrationTable() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
	 version VARCHAR(255) PRIMARY KEY,
	 applied_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
	 description TEXT
	);
	`

	_, err := m.db.ExecContext(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("error al crear tabla de migraciones: %w", err)
	}

	return nil
}

// RunMigrations ejecuta todas las migraciones pendientes
func (m *MigrationManager) RunMigrations() error {
	log.Println("Ejecutando migraciones pendientes...")

	// Obtener migraciones aplicadas
	appliedMigrations, err := m.getAppliedMigrations()
	if err != nil {
		return err
	}

	// Obtener archivos de migración disponibles
	migrationFiles, err := m.getMigrationFiles()
	if err != nil {
		return err
	}

	// Ordenar archivos de migración por nombre (que debe incluir versión)
	sort.Strings(migrationFiles)

	// Ejecutar migraciones pendientes
	for _, migrationFile := range migrationFiles {
		version := filepath.Base(migrationFile)
		// Quitar extensión .sql
		version = strings.TrimSuffix(version, filepath.Ext(version))

		// Comprobar si ya se aplicó esta migración
		if _, exists := appliedMigrations[version]; exists {
			log.Printf("Migración %s ya aplicada, omitiendo.", version)
			continue
		}

		// Ejecutar migración
		log.Printf("Aplicando migración: %s", version)
		if err := m.applyMigration(version, migrationFile); err != nil {
			return fmt.Errorf("error al aplicar migración %s: %w", version, err)
		}
	}

	log.Println("Migraciones completadas")
	return nil
}

// getAppliedMigrations obtiene las migraciones ya aplicadas desde la base de datos
func (m *MigrationManager) getAppliedMigrations() (map[string]bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := m.db.QueryContext(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, fmt.Errorf("error al consultar migraciones aplicadas: %w", err)
	}
	defer rows.Close()

	appliedMigrations := make(map[string]bool)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("error al leer versión de migración: %w", err)
		}
		appliedMigrations[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar sobre migraciones aplicadas: %w", err)
	}

	return appliedMigrations, nil
}

// getMigrationFiles obtiene la lista de archivos de migración disponibles
func (m *MigrationManager) getMigrationFiles() ([]string, error) {
	if _, err := os.Stat(m.scriptPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("directorio de migraciones no existe: %s", m.scriptPath)
		}
		return nil, fmt.Errorf("error al acceder al directorio de migraciones: %w", err)
	}

	var migrationFiles []string

	err := filepath.Walk(m.scriptPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Solo considerar archivos .sql
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".sql") {
			migrationFiles = append(migrationFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("error al listar archivos de migración: %w", err)
	}

	return migrationFiles, nil
}

// applyMigration aplica una migración específica
func (m *MigrationManager) applyMigration(version, filePath string) error {
	// Leer contenido del archivo de migración
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error al leer archivo de migración %s: %w", filePath, err)
	}

	// Iniciar transacción
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("error al iniciar transacción: %w", err)
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			log.Printf("Migración %s revertida debido a error", version)
		}
	}()

	// Ejecutar consultas SQL del archivo
	if _, err = tx.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("error al ejecutar migración: %w", err)
	}

	// Registrar migración como aplicada
	description := filepath.Base(filePath)
	if _, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations (version, description) VALUES ($1, $2)",
		version, description); err != nil {
		return fmt.Errorf("error al registrar migración: %w", err)
	}

	// Confirmar transacción
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("error al confirmar transacción: %w", err)
	}

	log.Printf("Migración %s aplicada correctamente", version)
	return nil
}
