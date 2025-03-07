-- V002_create_tasks.sql
-- Script para crear la estructura de Tasks con políticas de seguridad

-- Tabla principal para Tasks
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    metadata JSONB NOT NULL,
    spec JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    owner VARCHAR(100) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(100) NOT NULL DEFAULT 'default'
);

-- Trigger para actualizar automáticamente el campo updated_at
DROP TRIGGER IF EXISTS update_tasks_timestamp ON tasks;
CREATE TRIGGER update_tasks_timestamp
BEFORE UPDATE ON tasks
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

-- Índices para optimizar las consultas más comunes
CREATE INDEX IF NOT EXISTS idx_tasks_metadata_name ON tasks ((metadata->>'name'));
CREATE INDEX IF NOT EXISTS idx_tasks_spec_worker_id ON tasks ((spec->>'worker_id'));
CREATE INDEX IF NOT EXISTS idx_tasks_owner ON tasks (owner);
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_id ON tasks (tenant_id);

-- Habilitar Row-Level Security (RLS) en la tabla
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

-- Políticas RLS para tasks
DROP POLICY IF EXISTS admin_all_policy ON tasks;
CREATE POLICY admin_all_policy ON tasks
    USING (current_user_role() = 'admin'::user_role);

DROP POLICY IF EXISTS operator_tenant_policy ON tasks;
CREATE POLICY operator_tenant_policy ON tasks
    USING (
        current_user_role() = 'operator'::user_role AND
        tenant_id = current_user_tenant()
    );

DROP POLICY IF EXISTS viewer_tenant_policy ON tasks;
CREATE POLICY viewer_tenant_policy ON tasks
    FOR SELECT
    USING (
        current_user_role() = 'viewer'::user_role AND
        tenant_id = current_user_tenant()
    );

-- Vista para facilitar consultas con datos enriquecidos
CREATE OR REPLACE VIEW tasks_view AS
SELECT
    id,
    metadata->>'name' AS name,
    metadata->>'description' AS description,
    spec->>'worker_id' AS worker_id,
    spec->>'command' AS command,
    created_at,
    updated_at,
    owner,
    tenant_id
FROM
    tasks;

-- Función para validar la estructura del objeto metadata
CREATE OR REPLACE FUNCTION validate_task_metadata_json(metadata JSONB)
RETURNS BOOLEAN AS $$
BEGIN
  -- Verifica que metadata contiene campos requeridos
  IF NOT (metadata ? 'name') THEN
    RETURN FALSE;
  END IF;

  -- Validar que el nombre no está vacío
  IF metadata->>'name' = '' THEN
    RETURN FALSE;
  END IF;

  RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Función para validar la estructura del objeto spec
CREATE OR REPLACE FUNCTION validate_task_spec_json(spec JSONB)
RETURNS BOOLEAN AS $$
BEGIN
  -- Verifica que spec contiene campos requeridos
  IF NOT (spec ? 'command' AND spec ? 'params') THEN
    RETURN FALSE;
  END IF;

  -- Validar que el comando no está vacío
  IF jsonb_array_length(spec->'command') = 0 THEN
    RETURN FALSE;
  END IF;

  RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Añadir restricciones CHECK a la tabla
ALTER TABLE tasks
  ADD CONSTRAINT check_task_metadata_format CHECK (validate_task_metadata_json(metadata)),
  ADD CONSTRAINT check_task_spec_format CHECK (validate_task_spec_json(spec));

-- Comentarios en las tablas y columnas para documentación
COMMENT ON TABLE tasks IS 'Almacena las definiciones de tareas';
COMMENT ON COLUMN tasks.id IS 'Identificador único UUID';
COMMENT ON COLUMN tasks.metadata IS 'Metadatos de la tarea como nombre, descripción, etc.';
COMMENT ON COLUMN tasks.spec IS 'Especificación técnica de la tarea';
COMMENT ON COLUMN tasks.owner IS 'Usuario propietario de la tarea';
COMMENT ON COLUMN tasks.tenant_id IS 'ID del inquilino al que pertenece esta tarea';