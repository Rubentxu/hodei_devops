// M001_initialize_resource_pools_mongodb.js
// Script para inicializar las colecciones y validaciones en MongoDB para Resource Pools

// Usamos db.getSiblingDB para asegurarnos que usamos la BD correcta
var db = db.getSiblingDB('hodei');

// Crear colección de resource pools con esquema de validación JSON
db.createCollection('resource_pools', {
  validator: {
    $jsonSchema: {
      bsonType: 'object',
      required: ['id', 'metadata', 'spec', 'status', 'owner', 'tenant_id', 'created_at', 'updated_at'],
      properties: {
        id: {
          bsonType: 'string',
          description: 'ID único en formato UUID, campo requerido'
        },
        metadata: {
          bsonType: 'object',
          required: ['name'],
          properties: {
            name: {
              bsonType: 'string',
              description: 'Nombre del resource pool, campo requerido'
            },
            description: {
              bsonType: 'string',
              description: 'Descripción del resource pool, opcional'
            },
            labels: {
              bsonType: 'array',
              description: 'Etiquetas del resource pool, opcional',
              items: {
                bsonType: 'string'
              }
            },
            annotations: {
              bsonType: 'object',
              description: 'Anotaciones del resource pool, opcional'
            },
            createdAt: {
              bsonType: 'date',
              description: 'Fecha de creación'
            },
            updatedAt: {
              bsonType: 'date',
              description: 'Fecha de última actualización'
            }
          }
        },
        spec: {
          bsonType: 'object',
          required: ['poolID', 'type'],
          properties: {
            poolID: {
              bsonType: 'string',
              description: 'ID del pool, campo requerido'
            },
            type: {
              bsonType: 'string',
              enum: ['Kubernetes', 'Docker', 'VM'],
              description: 'Tipo del resource pool, debe ser uno de los valores permitidos'
            },
            config: {
              bsonType: 'object',
              description: 'Configuración extendida, opcional'
            }
          }
        },
        status: {
          bsonType: 'object',
          required: ['state'],
          properties: {
            state: {
              bsonType: 'string',
              description: 'Estado del resource pool, campo requerido'
            }
          }
        },
        owner: {
          bsonType: 'string',
          description: 'Propietario del recurso'
        },
        tenant_id: {
          bsonType: 'string',
          description: 'ID del inquilino al que pertenece este recurso'
        },
        created_at: {
          bsonType: 'date',
          description: 'Fecha de creación a nivel de documento'
        },
        updated_at: {
          bsonType: 'date', 
          description: 'Fecha de última actualización a nivel de documento'
        }
      }
    }
  }
});

// Crear índices para mejorar el rendimiento de las consultas frecuentes
db.resource_pools.createIndex({ id: 1 }, { unique: true });
db.resource_pools.createIndex({ 'metadata.name': 1 });
db.resource_pools.createIndex({ 'metadata.labels': 1 });
db.resource_pools.createIndex({ 'spec.poolID': 1 });
db.resource_pools.createIndex({ 'spec.type': 1 });
db.resource_pools.createIndex({ 'status.state': 1 });
db.resource_pools.createIndex({ owner: 1 });
db.resource_pools.createIndex({ tenant_id: 1 });
db.resource_pools.createIndex({ created_at: 1 });
db.resource_pools.createIndex({ updated_at: 1 });

// Crear función para validar tipo de pool (similar a validate_spec_json)
db.system.js.save({
  _id: 'validatePoolType',
  value: function(type) {
    const validTypes = ['Kubernetes', 'Docker', 'VM'];
    return validTypes.includes(type);
  }
});

// Crear usuarios demo
db.createCollection('users');
db.users.insertMany([
  {
    username: 'admin',
    role: 'admin',
    tenant_id: 'default',
    created_at: new Date()
  },
  {
    username: 'operator',
    role: 'operator',
    tenant_id: 'default',
    created_at: new Date()
  },
  {
    username: 'viewer',
    role: 'viewer',
    tenant_id: 'default',
    created_at: new Date()
  }
], { ordered: false });

// Función de ayuda para insertar resource pools de ejemplo
function insertSampleResourcePools() {
  const now = new Date();
  
  db.resource_pools.insertMany([
    {
      id: UUID().toString(),
      metadata: {
        name: 'Demo Kubernetes Cluster',
        description: 'Demo Kubernetes cluster for testing',
        labels: ['demo', 'kubernetes', 'test'],
        annotations: {
          env: 'development',
          purpose: 'testing'
        },
        createdAt: now,
        updatedAt: now
      },
      spec: {
        poolID: 'demo-k8s-1',
        type: 'Kubernetes',
        config: {
          version: '1.26',
          nodes: 3,
          region: 'eu-west-1'
        }
      },
      status: {
        state: 'Active'
      },
      owner: 'system',
      tenant_id: 'default',
      created_at: now,
      updated_at: now
    },
    {
      id: UUID().toString(),
      metadata: {
        name: 'Demo Docker Environment',
        description: 'Docker environment for development',
        labels: ['demo', 'docker', 'development'],
        annotations: {
          env: 'development',
          purpose: 'development'
        },
        createdAt: now,
        updatedAt: now
      },
      spec: {
        poolID: 'demo-docker-1',
        type: 'Docker',
        config: {
          version: '24.0',
          storage_driver: 'overlay2'
        }
      },
      status: {
        state: 'Active'
      },
      owner: 'system',
      tenant_id: 'default',
      created_at: now,
      updated_at: now
    }
  ], { ordered: false });
}

// Insertar algunos ejemplos (opcional, comenta si no deseas datos de ejemplo)
try {
  insertSampleResourcePools();
  print("Datos de ejemplo insertados correctamente");
} catch (error) {
  print("Error al insertar datos de ejemplo: " + error.message);
}

print("Inicialización de MongoDB para ResourcePools completada");