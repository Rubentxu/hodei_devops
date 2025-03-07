-- V003_create_workers.sql
-- Script para crear la estructura de Workers con políticas de seguridad

-- Tabla principal para Workers
CREATE TABLE IF NOT EXISTS workers (
    id UUID PRIMARY KEY,
    metadata JSONB NOT NULL,
    spec JSONB NOT NULL,
    status JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    owner VARCHAR(100) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(100) NOT NULL DEFAULT 'default'
);

-- Trigger para actualizar automáticamente el campo updated_at
DROP TRIGGER IF EXISTS update_workers_timestamp ON workers;
CREATE TRIGGER update_workers_timestamp
BEFORE UPDATE ON workers
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

-- Índices para optimizar las consultas más comunes
CREATE INDEX IF NOT EXISTS idx_workers_metadata_name ON workers ((metadata->>'name'));
CREATE INDEX IF NOT EXISTS idx_workers_spec_type ON workers ((spec->>'instance_type'));
CREATE INDEX IF NOT EXISTS idx_workers_status ON workers ((status->>'Status'));
CREATE INDEX IF NOT EXISTS idx_workers_owner ON workers (owner);
CREATE INDEX IF NOT EXISTS idx_workers_tenant_id ON workers (tenant_id);

-- Habilitar Row-Level Security (RLS) en la tabla
ALTER TABLE workers ENABLE ROW LEVEL SECURITY;

-- Políticas RLS para workers
DROP POLICY IF EXISTS admin_all_policy ON workers;
CREATE POLICY admin_all_policy ON workers
    USING (current_user_role() = 'admin'::user_role);

DROP POLICY IF EXISTS operator_tenant_policy ON workers;
CREATE POLICY operator_tenant_policy ON workers
    USING (
        current_user_role() = 'operator'::user_role AND
        tenant_id = current_user_tenant()
    );

DROP POLICY IF EXISTS viewer_tenant_policy ON workers;
CREATE POLICY viewer_tenant_policy ON workers
    FOR SELECT
    USING (
        current_user_role() = 'viewer'::user_role AND
        tenant_id = current_user_tenant()
    );

-- Vista para facilitar consultas con datos enriquecidos
CREATE OR REPLACE VIEW workers_view AS
SELECT
    id,
    metadata->>'name' AS name,
    metadata->>'description' AS description,
    spec->>'instance_type' AS type,
    spec->>'image' AS image,
    status->>'Status' AS status,
    created_at,
    updated_at,
    owner,
    tenant_id
FROM
    workers;

-- Función para validar la estructura del objeto metadata
CREATE OR REPLACE FUNCTION validate_worker_metadata_json(metadata JSONB)
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
CREATE OR REPLACE FUNCTION validate_worker_spec_json(spec JSONB)
RETURNS BOOLEAN AS $$
BEGIN
  -- Verifica que spec contiene campos requeridos
  IF NOT (spec ? 'instance_type') THEN
    RETURN FALSE;
  END IF;

  -- Validar que el tipo de instancia es válido
  IF NOT (spec->>'instance_type' IN ('docker', 'kubernetes', 'vm')) THEN
    RETURN FALSE;
  END IF;

  RETURN TRUE;
END;
$$ LANGUAGE plpgsql;

-- Añadir restricciones CHECK a la tabla
ALTER TABLE workers
  ADD CONSTRAINT check_worker_metadata_format CHECK (validate_worker_metadata_json(metadata)),
  ADD CONSTRAINT check_worker_spec_format CHECK (validate_worker_spec_json(spec));

-- Comentarios en las tablas y columnas para documentación
COMMENT ON TABLE workers IS 'Almacena las definiciones de workers';
COMMENT ON COLUMN workers.id IS 'Identificador único UUID';
COMMENT ON COLUMN workers.metadata IS 'Metadatos del worker como no