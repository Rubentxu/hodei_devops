# Etapa base
FROM golang:1.23 AS base

WORKDIR /app

RUN apt-get update && apt-get install -y protobuf-compiler && \
    curl -Lo /usr/local/bin/grpcurl https://github.com/fullstorydev/grpcurl/releases/download/v1.8.7/grpcurl_1.8.7_linux_x86_64 && \
    chmod +x /usr/local/bin/grpcurl


# Habilitar trazas detalladas
ENV GODEBUG=gocacheverify=1

# Copiar dependencias de Go y el código fuente
COPY tests/go.mod tests/go.sum ./

# Descarga las dependencias
RUN go mod download -x

# Copiar el resto del código de pruebas
COPY tests/ ./

# Etapa de testeo
FROM base AS tests

# Configuración de entorno
ENV CGO_ENABLED=0 \
    GOOS=linux

# Compilar el ejecutable de pruebas
RUN go build -o /tests-runner main.go

# Etapa final
FROM alpine:3.18

# Instalar herramientas necesarias (curl para healthchecks)
RUN apk add --no-cache ca-certificates curl

# Copiar binario de pruebas y recursos necesarios
WORKDIR /app
COPY --from=tests /tests-runner /app/tests-runner
COPY ./certs /certs

# Exponer variables de entorno relevantes
ENV JWT_SECRET="" \
    JWT_TOKEN="" \
    GRPC_SERVER_ADDRESS="server:50051" \
    API_BASE_URL="http://client:8080"

# Comando por defecto para ejecutar las pruebas
CMD ["/app/tests-runner"]
