-- V004_create_task_executions.sql
-- Script para crear la estructura de Task Executions con políticas de seguridad

-- Tabla principal para Task Executions
CREATE TABLE IF NOT EXISTS task_executions (
    id UUID PRIMARY KEY,
    metadata JSONB NOT NULL,
    task_id UUID NOT NULL,
    worker_id UUID NOT NULL,
    status JSONB NOT NULL,
    input_args JSONB DEFAULT '[]',
    owner VARCHAR(100) NOT NULL DEFAULT 'system',
    tenant_id VARCHAR(100) NOT NULL DEFAULT 'default',
    CONSTRAINT fk_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    CONSTRAINT fk_worker FOREIGN KEY (worker_id) REFERENCES workers(id) ON DELETE RESTRICT
);

-- Trigger para actualizar automáticamente el campo updated_at
DROP TRIGGER IF EXISTS update_task_executions_timestamp ON task_executions;
CREATE TRIGGER update_task_executions_timestamp
BEFORE UPDATE ON task_executions
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();

-- Índices para optimizar las consultas más comunes
CREATE INDEX IF NOT EXISTS idx_task_executions_task_id ON task_executions (task_id);
CREATE INDEX IF NOT EXISTS idx_task_executions_worker_id ON task_executions (worker_id);
CREATE INDEX IF NOT EXISTS idx_task_executions_status ON task_executions ((status->>'state'));
CREATE INDEX IF NOT EXISTS idx_task_executions_start_time ON task_executions ((status->>'start_time'));
CREATE INDEX IF NOT EXISTS idx_task_executions_tenant_id ON task_executions (tenant_id);
CREATE INDEX IF NOT EXISTS idx_task_executions_owner ON task_executions (owner);

-- Habilitar Row-Level Security (RLS) en la tabla
ALTER TABLE task_executions ENABLE ROW LEVEL SECURITY;

-- Políticas RLS para task_executions
DROP POLICY IF EXISTS admin_all_policy ON task_executions;
CREATE POLICY admin_all_policy ON task_executions
    USING (current_user_role() = 'admin'::user_role);

DROP POLICY IF EXISTS operator_tenant_policy ON task_executions;
CREATE POLICY operator_tenant_policy ON task_executions
    USING (
        current_user_role() = 'operator'::user_role AND
        tenant_id = current_user_tenant()
    );

DROP POLICY IF EXISTS viewer_tenant_policy ON task_executions;
CREATE POLICY viewer_tenant_policy ON task_executions
    FOR SELECT
    USING (
        current_user_role() = 'viewer'::user_role AND
        tenant_id = current_user_tenant()
    );

-- Vista para facilitar consultas con datos enriquecidos
CREATE OR REPLACE VIEW task_executions_view AS
SELECT
    te.id,
    te.metadata->>'name' AS name,
    te.metadata->>'description' AS description,
    te.task_id,
    t.metadata->>'name' AS task_name,
    te.worker_id,
    w.metadata->>'name' AS worker_name,
    te.status->>'state' AS state,
    te.status->>'message' AS message,
    (te.status->>'start_time')::TIMESTAMP WITH TIME ZONE AS start_time,
    (te.status->>'end_time')::TIMESTAMP WITH TIME ZONE AS end_time,
    te.owner,
    te.tenant_id
FROM
    task_executions te
JOIN
    tasks t ON te.task_id = t.id
JOIN
    workers w ON te.worker_id = w.id;

-- Comentarios en las tablas y columnas para documentación
COMMENT ON TABLE task_executions IS 'Almacena las ejecuciones de tareas';
COMMENT ON COLUMN task_executions.id IS 'Identificador único UUID';
COMMENT ON COLUMN task_executions.metadata IS 'Metadatos de la ejecución como nombre, descripción, etc.';
COMMENT ON COLUMN task_executions.task_id IS 'ID de la tarea asociada';
COMMENT ON COLUMN task_executions.worker_id IS 'ID del worker utilizado para la ejecución';
COMMENT ON COLUMN task_executions.status IS 'Estado de la ejecución incluyendo estado, tiempos y mensajes';
COMMENT ON COLUMN task_executions.input_args IS 'Argumentos de entrada para la ejecución';
COMMENT ON COLUMN task_executions.owner IS 'Usuario propietario de la ejecución';
COMMENT ON COLUMN task_executions.tenant_id IS 'ID del inquilino al que pertenece esta ejecución';