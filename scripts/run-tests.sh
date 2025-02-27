#!/bin/bash

# Verificar si la imagen de remote-process existe
if ! docker image inspect $WORKER_IMAGE >/dev/null 2>&1; then
    echo "Error: La imagen $WORKER_IMAGE no existe localmente."
    echo "Por favor, construye la imagen primero usando 'make build-remote-process'"
    exit 1
fi

# Run tests in verbose mode, showing detailed logs and summary of results
set -e

echo "Starting tests with verbose output..."

# Run all tests with verbose flag
go test -v ./...

# Optionally, generate a test report file if needed
# go test -v ./... | tee test-report.txt

# End of tests