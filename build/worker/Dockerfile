# Build stage
FROM golang:1.23 AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y protobuf-compiler tree
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# Copiar solo los archivos necesarios para la compilación
COPY protos/ ./protos/
COPY worker/ ./worker/

# Generar código protobuf
RUN protoc --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    protos/remote_process/remote_process.proto

# Descargar dependencias y compilar
WORKDIR /app/protos
RUN go mod download

WORKDIR /app/worker
RUN go mod download

WORKDIR /app/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker cmd/main.go

# Runtime stage
FROM alpine:3.18

RUN apk add --no-cache ca-certificates curl

WORKDIR /app

# Copiar el binario compilado
COPY --from=builder /worker .

# Crear directorio para certificados
RUN mkdir -p /certs

# Puerto HTTP
EXPOSE 8080

# Comando para ejecutar el workere
CMD ["./worker"]