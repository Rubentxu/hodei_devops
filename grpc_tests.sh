#!/bin/bash
set -e

# --- Configuración ---
GRPC_SERVER_ADDRESS="localhost:50051"
CERT_DIR="certs/dev"
SERVER_CERT="${CERT_DIR}/remote_process-cert.pem"
SERVER_KEY="${CERT_DIR}/remote_process-key.pem"
CA_CERT="${CERT_DIR}/ca-cert.pem"
JWT_SECRET="test_secret_key_for_development_1234567890"
MAX_RETRIES=5
RETRY_DELAY=3
BIN_DIR="bin"
SERVER_PID=""
TMP_OUTPUT="/tmp/grpc_test_output"
TEST_RESULTS=()

# --- Formato del reporte ---
header() {
    echo -e "\n\033[1;34m$1\033[0m"
}

success() {
    echo -e "\033[1;32m✓ $1\033[0m"
}

error() {
    echo -e "\033[1;31m✗ $1\033[0m"
}

# --- Generar token JWT (corregido) ---
gen_jwt() {
    local header=$(echo -n '{"alg":"HS256","typ":"JWT"}' | base64 -w 0 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
    local payload=$(echo -n '{"sub":"test-user","role":"admin","exp":4683864000}' | base64 -w 0 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
    local signature=$(echo -n "${header}.${payload}" | openssl dgst -binary -sha256 -hmac "${JWT_SECRET}" | base64 -w 0 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
    echo "${header}.${payload}.${signature}"
}

# --- Compilar servidor ---
build_server() {
    header "Compilando servidor..."
    mkdir -p "${BIN_DIR}"
    go build -o "${BIN_DIR}/remote_process" remote_process/cmd/main.go
}

# --- Iniciar servidor (corregido) ---
start_server() {
    header "Iniciando servidor..."
    APPLICATION_PORT="50051" \
    ENV="development" \
    SERVER_CERT_PATH="${SERVER_CERT}" \
    SERVER_KEY_PATH="${SERVER_KEY}" \
    CA_CERT_PATH="${CA_CERT}" \
    JWT_SECRET="${JWT_SECRET}" \
    "${BIN_DIR}/remote_process" > "${BIN_DIR}/server.log" 2>&1 &
    SERVER_PID=$!
    echo "PID: $SERVER_PID | Logs: ${BIN_DIR}/server.log"
}

# --- Verificar salud ---
check_health() {
    local jwt_token=$1
    header "Verificando salud del servidor..."

    for ((i=1; i<=MAX_RETRIES; i++)); do
        if grpcurl -d '{}' \
            -H "authorization: ${jwt_token}" \
            -cacert "${CA_CERT}" \
            -cert "${CERT_DIR}/worker-cert.pem" \
            -key "${CERT_DIR}/worker-key.pem" \
            "${GRPC_SERVER_ADDRESS}" \
            grpc.health.v1.Health/Check >/dev/null 2>&1; then
            success "Servidor listo!"
            return 0
        else
            echo "Intento $i/$MAX_RETRIES: Servidor no responde"
            sleep $RETRY_DELAY
        fi
    done
    error "Servidor no disponible"
    exit 1
}

# --- Ejecutar pruebas ---
run_tests() {
    local jwt_token=$(gen_jwt)
    header "Información del Token JWT:"
    echo -e "Secreto: \033[35m${JWT_SECRET}\033[0m"
    echo -n "Payload: " && echo "${jwt_token}" | cut -d '.' -f 2 | base64 -d 2>/dev/null
    echo -e "Token: \033[35m${jwt_token:0:50}...\033[0m\n"

     declare -a COMMANDS=(
         # --- 1. Comandos Básicos ---
         '{"command": ["echo", "Test Básico"], "environment": {}, "working_directory": "/tmp", "process_id": "basic-1"}'
         '{"command": ["pwd"], "environment": {}, "working_directory": "/etc", "process_id": "basic-2"}'

         # --- 2. Expansión Variables ---
         '{"command": ["bash", "-c", "echo $USER"], "environment": {"USER": "test_user"}, "working_directory": "/tmp", "process_id": "var-1"}'
         '{"command": ["bash", "-c", "echo ${VAR_CON_ESPACIOS}"], "environment": {"VAR_CON_ESPACIOS": "Hola Mundo"}, "working_directory": "/tmp", "process_id": "var-2"}'

         # --- 3. Manejo de Errores ---
         '{"command": ["comando_inexistente"], "environment": {}, "working_directory": "/tmp", "process_id": "error-1"}'
         '{"command": ["ls", "/directorio/inexistente"], "environment": {}, "working_directory": "/tmp", "process_id": "error-2"}'

         # --- 4. Comandos Complejos ---
         '{"command": ["bash", "-c", "for i in {1..5}; do echo \"Iteración $i\"; sleep 0.1; done"], "environment": {}, "working_directory": "/tmp", "process_id": "complex-1"}'
         '{"command": ["bash", "-c", "curl -s ifconfig.me"], "environment": {}, "working_directory": "/tmp", "process_id": "complex-2"}'

         # --- 5. Operaciones Filesystem ---
         '{"command": ["bash", "-c", "mkdir -p test_dir && touch test_dir/file{1..3}.txt"], "environment": {}, "working_directory": "/tmp", "process_id": "fs-1"}'
         '{"command": ["bash", "-c", "tar -czf archive.tar.gz test_dir"], "environment": {}, "working_directory": "/tmp", "process_id": "fs-2"}'

         # --- 6. Procesos Concurrentes ---
         '{"command": ["bash", "-c", "for i in {1..3}; do echo \"Proceso $i\" & done; wait"], "environment": {}, "working_directory": "/tmp", "process_id": "concurrent-1"}'

         # --- 7. Manejo de Señales ---
#         '{"command": ["bash", "-c", "trap \"echo SEÑAL RECIBIDA; exit 0\" SIGTERM; while true; do sleep 1; done"], "environment": {}, "working_directory": "/tmp", "process_id": "signal-1"}'

         # --- 8. Entrada/Salida ---
         '{"command": ["bash", "-c", "read input && echo \"Leíste: $input\""], "environment": {}, "working_directory": "/tmp", "process_id": "io-1"}'
     )


    for CMD in "${COMMANDS[@]}"; do
        local process_id=$(echo "$CMD" | jq -r .process_id)
        local command=$(echo "$CMD" | jq -r '.command | join(" ")')
        echo -e "\n\033[1;36mTEST: ${process_id}\033[0m | Comando: \033[33m${command}\033[0m"

        grpcurl -d "$CMD" \
            -H "authorization: bearer ${jwt_token}" \
            -cacert "${CA_CERT}" \
            -cert "${CERT_DIR}/worker-cert.pem" \
            -key "${CERT_DIR}/worker-key.pem" \
            "${GRPC_SERVER_ADDRESS}" \
            remote_process.RemoteProcessService/StartProcess > "${TMP_OUTPUT}" 2>&1

        echo -e "\033[1;36mRespuesta:\033[0m"
        echo -e "\033[34m$(cat "${TMP_OUTPUT}" | jq -r .)\033[0m"

        if grep -q "Code:" "${TMP_OUTPUT}"; then
            error "Fallo en ${process_id}"
            echo -e "\033[31m$(cat "${TMP_OUTPUT}")\033[0m"
            TEST_RESULTS+=("FAIL: ${process_id}")
        else
            success "Éxito en ${process_id}"
            echo -e "\033[32mSalida:\n$(cat "${TMP_OUTPUT}" | jq -r .output)\033[0m"
            TEST_RESULTS+=("PASS: ${process_id}")
        fi
    done
}

# --- Generar reporte final ---
generate_report() {
    header "\nResultados de las Pruebas:"
    for result in "${TEST_RESULTS[@]}"; do
        if [[ "$result" == PASS* ]]; then
            echo -e "\033[32m${result}\033[0m"
        else
            echo -e "\033[31m${result}\033[0m"
        fi
    done
}

# --- Limpieza ---
cleanup() {
    header "Deteniendo servidor..."
    [[ -n "$SERVER_PID" ]] && kill "$SERVER_PID" 2>/dev/null
    rm -f "${TMP_OUTPUT}"
}

# --- Manejar Ctrl+C ---
trap cleanup EXIT

# --- Flujo principal ---
main() {
    build_server
    start_server
    local jwt_token=$(gen_jwt)
    check_health "$jwt_token"
    run_tests
    generate_report
}

main