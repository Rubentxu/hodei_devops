# Etapa base
FROM golang:1.23 AS base

# Exponer variables de entorno relevantes
ENV JWT_SECRET="" \
    JWT_TOKEN="" \
    GRPC_SERVER_ADDRESS="remote-process:50051" \
    API_BASE_URL="http://worker:8080"

WORKDIR /app

RUN apt-get update && apt-get install -y protobuf-compiler && \
    curl -Lo /usr/local/bin/grpcurl https://github.com/fullstorydev/grpcurl/releases/download/v1.8.7/grpcurl_1.8.7_linux_x86_64 && \
    chmod +x /usr/local/bin/grpcurl

# Habilitar trazas detalladas
ENV GODEBUG=gocacheverify=1

# Copiar dependencias de Go y el código fuente
COPY tests/go.mod tests/go.sum ./
COPY certs/dev /certs

# Descargar dependencias
RUN go mod download -x

# Copiar el resto del código de pruebas
COPY tests/ /app/tests/

# Configuración de entorno
ENV CGO_ENABLED=0 \
    GOOS=linux

# Comprobación de archivos y directorios antes de continuar
RUN test -f tests/go.mod || (echo "ERROR: tests/go.mod no existe" && exit 1)
RUN test -f tests/go.sum || (echo "ERROR: tests/go.sum no existe" && exit 1)
RUN test -d /certs || (echo "ERROR: El directorio 'certs' no existe" && exit 1)

# Comprobación de archivos de tests
RUN if ! find /app/tests -type f -name '*_test.go' | grep -q .; then \
        echo "ERROR: No se encontraron archivos de tests en el directorio '/app/tests'"; \
        exit 1; \
    fi

# Cambiar el directorio de trabajo al módulo de tests
WORKDIR /app/tests

# Ejecutar los tests desde el directorio de tests
CMD ["sh", "-c", "go test -v ./... -test.v -test.bench=. 2>&1"]


