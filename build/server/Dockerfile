# Build stage
FROM golang:1.23 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y protobuf-compiler tree
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copiar solo los archivos necesarios para la compilación
COPY protos/ ./protos/
COPY remote_process/ ./remote_process/


# Generar código protobuf
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    protos/remote_process/remote_process.proto

# Descargar dependencias y compilar
WORKDIR /app/protos
RUN go mod download

WORKDIR /app/remote_process
RUN go mod download

WORKDIR /app/remote_process
RUN CGO_ENABLED=0 GOOS=linux go build -o /server cmd/main.go


# Runtime stage
FROM alpine:3.18

# Instalar dependencias necesarias
RUN apk add --no-cache ca-certificates curl bash

# Descargar y extraer grpcurl
RUN curl -L -o grpcurl.tar.gz https://github.com/fullstorydev/grpcurl/releases/download/v1.9.2/grpcurl_1.9.2_linux_x86_64.tar.gz && \
    tar -xzf grpcurl.tar.gz -C /usr/local/bin && \
    rm grpcurl.tar.gz

WORKDIR /app

# Copiar el binario compilado
COPY --from=builder /server .

# Copiar el script de health check al contenedor
COPY grpc_health_check.sh .
# Asegurar permisos de ejecución
RUN chmod +x grpc_health_check.sh

# Crear directorio para certificados
RUN mkdir -p /certs

# Puerto gRPC
EXPOSE 50051

# Comando para ejecutar el servidor
CMD ["./server"]
