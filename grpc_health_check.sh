#!/bin/sh

# Instalar grpcurl (si es necesario)
# go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

# Configurar la URL del servidor gRPC desde una variable de entorno con un valor predeterminado
GRPC_SERVER_ADDRESS=${GRPC_SERVER_ADDRESS:-"localhost:50051"}

echo "Checking gRPC health status"
echo "JWT_TOKEN: ${JWT_TOKEN}"
echo "GRPC_SERVER_ADDRESS: ${GRPC_SERVER_ADDRESS}"
ls -la /certs

grpcurl \
  -d '{}' \
  -rpc-header "authorization: Bearer ${JWT_TOKEN}" \
  -cacert /certs/ca-cert.pem \
  -cert /certs/worker-client-cert.pem \
  -key /certs/worker-client-key.pem \
  ${GRPC_SERVER_ADDRESS} grpc.health.v1.Health/Check || exit 1
