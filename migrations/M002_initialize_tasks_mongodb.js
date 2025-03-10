// MongoDB initialization script for Tasks collection
db = db.getSiblingDB('hodei');

// Drop collection if exists for clean initialization
if (db.getCollectionNames().indexOf("tasks") !== -1) {
    print("Dropping existing tasks collection");
    db.tasks.drop();
}

// Create tasks collection with validation
db.createCollection("tasks", {
    validator: {
        $jsonSchema: {
            bsonType: "object",
            required: ["id", "metadata", "spec"],
            properties: {
                id: {
                    bsonType: "string",
                    description: "must be a string and is required"
                },
                metadata: {
                    bsonType: "object",
                    required: ["name", "createdAt", "updatedAt"],
                    properties: {
                        name: {
                            bsonType: "string",
                            description: "must be a string and is required"
                        },
                        description: {
                            bsonType: "string",
                            description: "must be a string"
                        },
                        labels: {
                            bsonType: "array",
                            description: "must be an array of strings",
                            items: {
                                bsonType: "string"
                            }
                        },
                        annotations: {
                            bsonType: "object",
                            description: "must be an object with string keys and values"
                        },
                        createdAt: {
                            bsonType: "date",
                            description: "must be a date and is required"
                        },
                        updatedAt: {
                            bsonType: "date",
                            description: "must be a date and is required"
                        }
                    }
                },
                spec: {
                    bsonType: "object",
                    required: ["worker_id", "command", "params"],
                    properties: {
                        worker_id: {
                            bsonType: "string",
                            description: "must be a string and is required"
                        },
                        command: {
                            bsonType: "array",
                            description: "must be an array of strings",
                            items: {
                                bsonType: "string"
                            }
                        },
                        params: {
                            bsonType: "array",
                            description: "must be an array of parameter definitions",
                            items: {
                                bsonType: "object",
                                required: ["key", "type", "label"],
                                properties: {
                                    key: {
                                        bsonType: "string",
                                        description: "parameter key"
                                    },
                                    type: {
                                        bsonType: "string",
                                        description: "parameter type"
                                    },
                                    label: {
                                        bsonType: "string",
                                        description: "display label"
                                    },
                                    description: {
                                        bsonType: "string"
                                    },
                                    required: {
                                        bsonType: "bool"
                                    },
                                    default: {
                                        description: "default value of any type"
                                    },
                                    group: {
                                        bsonType: "string"
                                    },
                                    order: {
                                        bsonType: "int"
                                    },
                                    validations: {
                                        bsonType: "object"
                                    },
                                    options: {
                                        bsonType: "array",
                                        items: {
                                            bsonType: "object"
                                        }
                                    },
                                    depends: {
                                        bsonType: "object"
                                    }
                                }
                            }
                        },
                        param_values: {
                            bsonType: "object",
                            description: "parameter values as key-value pairs"
                        }
                    }
                }
            }
        }
    }
});

// Create unique index on id field
db.tasks.createIndex({ "id": 1 }, { unique: true });

// Create indices for common queries
db.tasks.createIndex({ "metadata.name": 1 });
db.tasks.createIndex({ "metadata.labels": 1 });
db.tasks.createIndex({ "spec.worker_id": 1 });

// Insert sample tasks for demonstration and testing
db.tasks.insertMany([
    {
        id: "11111111-1111-1111-1111-111111111111",
        metadata: {
            name: "Sample Task 1",
            description: "Example task for demonstration",
            labels: ["sample", "demo"],
            annotations: {
                environment: "development",
                category: "example"
            },
            createdAt: new Date(),
            updatedAt: new Date()
        },
        spec: {
            worker_id: "22222222-2222-2222-2222-222222222222",
            command: ["echo", "Hello World"],
            params: [
                {
                    key: "message",
                    type: "string",
                    label: "Message",
                    description: "Message to echo",
                    required: true,
                    default: "Hello World",
                    group: "basic",
                    order: 1
                },
                {
                    key: "iterations",
                    type: "integer",
                    label: "Iterations",
                    description: "Number of times to repeat",
                    required: false,
                    default: 1,
                    group: "advanced",
                    order: 2,
                    validations: {
                        min: 1,
                        max: 10
                    }
                }
            ],
            param_values: {
                message: "Custom message",
                iterations: 3
            }
        }
    },
    {
        id: "33333333-3333-3333-3333-333333333333",
        metadata: {
            name: "Deploy Application",
            description: "Deploy application to Kubernetes cluster",
            labels: ["deployment", "kubernetes"],
            annotations: {
                environment: "production",
                component: "backend"
            },
            createdAt: new Date(),
            updatedAt: new Date()
        },
        spec: {
            worker_id: "44444444-4444-4444-4444-444444444444",
            command: ["kubectl", "apply", "-f"],
            params: [
                {
                    key: "namespace",
                    type: "string",
                    label: "Namespace",
                    description: "Kubernetes namespace",
                    required: true,
                    default: "default",
                    group: "kubernetes",
                    order: 1
                },
                {
                    key: "manifest",
                    type: "file",
                    label: "Manifest File",
                    description: "Kubernetes manifest file",
                    required: true,
                    group: "kubernetes",
                    order: 2
                },
                {
                    key: "wait",
                    type: "boolean",
                    label: "Wait for completion",
                    description: "Wait for deployment to complete",
                    required: false,
                    default: true,
                    group: "options",
                    order: 3
                }
            ],
            param_values: {
                namespace: "production",
                manifest: "/deployments/app.yaml",
                wait: true
            }
        }
    }
]);

print("Tasks collection initialized successfully with sample data!");