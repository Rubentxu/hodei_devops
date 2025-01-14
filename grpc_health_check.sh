#!/bin/sh
# instalar grpcurl
# go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

echo "Checking gRPC health status"
echo "JWT_TOKEN: ${JWT_TOKEN}"
ls -la /certs

grpcurl \
  -d '{}' \
  -rpc-header "authorization: Bearer ${JWT_TOKEN}" \
  -cacert /certs/ca-cert.pem \
  -cert /certs/client-cert.pem \
  -key /certs/client-key.pem \
  localhost:50051 grpc.health.v1.Health/Check || exit 1

