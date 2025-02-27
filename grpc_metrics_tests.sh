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
TMP_OUTPUT="/tmp/grpc_metrics_test_output"
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

# --- Generar token JWT ---
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

# --- Iniciar servidor ---
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
            -cert "${CERT_DIR}/worker-client-cert.pem" \
            -key "${CERT_DIR}/worker-client-key.pem" \
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

# --- Ejecutar pruebas de métricas ---
run_metrics_tests() {
    local jwt_token=$(gen_jwt)
    header "Información del Token JWT:"
    echo -e "Secreto: \033[35m${JWT_SECRET}\033[0m"
    echo -n "Payload: " && echo "${jwt_token}" | cut -d '.' -f 2 | base64 -d 2>/dev/null
    echo -e "Token: \033[35m${jwt_token:0:50}...\033[0m\n"

    declare -a METRICS_REQUESTS=(
        '{"worker_id": "worker-1", "metric_types": ["CPU", "MEMORY"], "interval": 2}'
        '{"worker_id": "worker-2", "metric_types": ["DISK", "NETWORK"], "interval": 2}'
    )

    for REQUEST in "${METRICS_REQUESTS[@]}"; do
        local worker_id=$(echo "$REQUEST" | jq -r .worker_id)
        echo -e "\n\033[1;36mTEST: ${worker_id}\033[0m | Solicitud: \033[33m${REQUEST}\033[0m"

        # Usar timeout para limitar la duración del streaming
        grpcurl -v -d "$REQUEST" \
            -H "authorization: bearer ${jwt_token}" \
            -cacert "${CA_CERT}" \
            -cert "${CERT_DIR}/worker-client-cert.pem" \
            -key "${CERT_DIR}/worker-client-key.pem" \
            -max-time 60 \
            "${GRPC_SERVER_ADDRESS}" \
            remote_process.RemoteProcessService/CollectMetrics 2>&1 | tee "${TMP_OUTPUT}" &

        # Mostrar las trazas en tiempo real
        tail -f "${TMP_OUTPUT}" &

        # Esperar a que grpcurl termine
        wait

        echo -e "\033[1;36mRespuesta:\033[0m"
        echo -e "\033[34m$(cat "${TMP_OUTPUT}" | jq -r .)\033[0m"

        # Verificar si hay errores en la salida
        if grep -q "Error:" "${TMP_OUTPUT}" || grep -q "Failed:" "${TMP_OUTPUT}"; then
            error "Fallo en ${worker_id}"
            TEST_RESULTS+=("FAIL: ${worker_id}")
            cat "${TMP_OUTPUT}"
        else
            success "Éxito en ${worker_id}"
            TEST_RESULTS+=("PASS: ${worker_id}")
            echo -e "\033[1;36mÚltimas métricas recibidas:\033[0m"
            tail -n 10 "${TMP_OUTPUT}"
        fi

        # Limpiar procesos en segundo plano
        pkill -f "grpcurl.*CollectMetrics" || true
        wait
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
    run_metrics_tests
    generate_report
}

main