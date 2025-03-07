-- V001_create_resource_pools.sql
-- Script para crear la estructura de ResourcePoolDef con políticas de seguridad

-- Habilitar la extensión pgcrypto para generar UUIDs
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Tabla principal para Resource Pools
CREATE TABLE IF NOT EXISTS resource_pools (
    id UUID PRIMARY KEY,
    metadata JSONB NOT NULL,
    spec JSONB NOT NULL,
    status JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    owner VARCHAR(100) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(100) NOT NULL DEFAULT 'default'
);

-- Función para actualizar automáticamente el campo updated_at
CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger para actualizar automáticamente el campo updated_at
DROP TRIGGER IF EXISTS update_resource_pools_timestamp ON resource_pools;
CREATE TRIGGER update_resource_pools_timestamp
BEFORE UPDATE ON resource_pools
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

-- Índices para optimizar las consultas más comunes
CREATE INDEX IF NOT EXISTS idx_resource_pools_metadata_name ON resource_pools ((metadata->>'name'));
CREATE INDEX IF NOT EXISTS idx_resource_pools_spec_type ON resource_pools ((spec->>'type'));
CREATE INDEX IF NOT EXISTS idx_resource_pools_spec_pool_id ON resource_pools ((spec->>'poolID'));
CREATE INDEX IF NOT EXISTS idx_resource_pools_status_state ON resource_pools ((status->>'state'));
CREATE INDEX IF NOT EXISTS idx_resource_pools_owner ON resource_pools (owner);
CREATE INDEX IF NOT EXISTS idx_resource_pools_tenant_id ON resource_pools (tenant_id);

-- Habilitar Row-Level Security (RLS) en la tabla
ALTER TABLE resource_pools ENABLE ROW LEVEL SECURITY;

-- Crear tipos de roles para las políticas
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
        CREATE TYPE user_role AS ENUM ('admin', 'operator', 'viewer');
    END IF;
END$$;

-- Tabla de usuarios para gestionar roles y permisos
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) UNIQUE NOT NULL,
    role user_role NOT NULL DEFAULT 'viewer',
    tenant_id VARCHAR(100) NOT NULL DEFAULT 'default',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Insertar usuarios por defecto
INSERT INTO users (username, role, tenant_id)
VALUES
('admin', 'admin', 'default'),
('operator', 'operator', 'default'),
('viewer', 'viewer', 'default')
ON CONFLICT (username) DO NOTHING;

-- Función para obtener el rol y tenant del usuario actual
CREATE OR REPLACE FUNCTION current_user_role()
RETURNS user_role AS $$
DECLARE
    user_role_value user_role;
BEGIN
    SELECT role INTO user_role_value FROM users WHERE username = current_user;
    RETURN COALESCE(user_role_value, 'viewer'::user_role);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

CREATE OR REPLACE FUNCTION current_user_tenant()
RETURNS VARCHAR AS $$
DECLARE
    tenant VARCHAR;
BEGIN
    SELECT tenant_id INTO tenant FROM users WHERE username = current_user;
    RETURN COALESCE(tenant, 'default');
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Políticas RLS para resource_pools
-- Política para administradores (pueden ver y modificar todo)
CREATE POLICY admin_all_policy ON resource_pools
    USING (current_user_role() = 'admin'::user_role);

-- Política para operadores (pueden gestionar pools de su tenant)
CREATE POLICY operator_tenant_policy ON resource_pools
    USING (
        current_user_role() = 'operator'::user_role AND
        tenant_id = current_user_tenant()
    );

-- Política para observadores (solo lectura de pools de su tenant)
CREATE POLICY viewer_tenant_policy ON resource_pools
    FOR SELECT
    USING (
        current_user_role() = 'viewer'::user_role AND
        tenant_id = current_user_tenant()
    );

-- Vista para facilitar consultas con datos enriquecidos
CREATE OR REPLACE VIEW resource_pools_view AS
SELECT
    id,
    metadata->>'name' AS name,
    metadata->>'description' AS description,
    spec->>'type' AS type,
    spec->>'poolID' AS pool_id,
    status->>'state' AS state,
    created_at,
    updated_at,
    owner,
    tenant_id
FROM
    resource_pools;

-- Crear una función para insertar un pool con valores predeterminados
CREATE OR REPLACE FUNCTION create_resource_pool(
    metadata_json JSONB,
    spec_json JSONB,
    status_json JSONB DEFAULT '{"state": "PENDING"}'::JSONB
) RETURNS UUID AS $$
DECLARE
    new_id UUID;
BEGIN
    new_id := gen_random_uuid();

    INSERT INTO resource_pools (
        id,
        metadata,
        spec,
        status,
        owner,
        tenant_id
    ) VALUES (
        new_id,
        metadata_json,
        spec_json,
        status_json,
        current_user,
        current_user_tenant()
    );

    RETURN new_id;
END;
$$ LANGUAGE plpgsql;

-- Comentarios en las tablas y columnas para documentación
COMMENT ON TABLE resource_pools IS 'Almacena las definiciones de pools de recursos';
COMMENT ON COLUMN resource_pools.id IS 'Identificador único UUID';
COMMENT ON COLUMN resource_pools.metadata IS 'Metadatos del recurso como nombre, descripción, etc.';
COMMENT ON COLUMN resource_pools.spec IS 'Especificación técnica del pool de recursos';
COMMENT ON COLUMN resource_pools.status IS 'Estado actual del recurso';
COMMENT ON COLUMN resource_pools.owner IS 'Usuario propietario del recurso';
COMMENT ON COLUMN resource_pools.tenant_id IS 'ID del inquilino al que pertenece este recurso';

COMMENT ON TABLE users IS 'Usuarios del sistema con sus roles y permisos';
COMMENT ON COLUMN users.username IS 'Nombre de usuario único';
COMMENT ON COLUMN users.role IS 'Rol del usuario: admin, operator o viewer';
COMMENT ON COLUMN users.tenant_id IS 'ID del inquilino al que pertenece el usuario';

        ade esta sección después de la definición de la tabla resource_pools

-- Función para validar la estructura del objeto metadata
CREATE OR REPLACE FUNCTION validate_metadata_json(metadata JSONB)
RETURNS BOOLEAN AS $$
BEGIN
  -- Verifica que metadata contiene campos requeridos
  IF NOT (metadata ? 'name') THEN
    RETURN FALSE;
END IF;

  -- Validar que el nombre no está vacío
  IF metadata->>'name' = '' THEN