# Runtime stage only
FROM ubuntu:24.04

RUN apt-get update && \
    apt-get install -y \
    ca-certificates \
    curl \
    wget \
    bash && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copiar el binario desde la etapa de construcción o desde el directorio local
COPY bin/orchestrator /app/orchestrator

# Conceder permisos de ejecución al binario
RUN chmod +x /app/orchestrator
RUN test -f /app/orchestrator || exit 1

# Crear directorios necesarios
RUN mkdir -p /certs /app/test_pb_data

# Variables de entorno para TLS y JWT
ENV CLIENT_CERT_PATH="/certs/worker-client-cert.pem" \
    CLIENT_KEY_PATH="/certs/worker-client-key.pem" \
    CA_CERT_PATH="/certs/ca-cert.pem" \
    JWT_SECRET="" \
    JWT_TOKEN=""

# Puerto HTTP
EXPOSE 8090

# Se usa una ruta absoluta para el directorio de datos
ENTRYPOINT ["/app/orchestrator", "serve", "--dir=/app/test_pb_data", "--http=0.0.0.0:8090"]