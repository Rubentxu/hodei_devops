#!/bin/bash
set -e

# ----------------------
# CONFIGURACIÓN BÁSICA
# ----------------------
GRPC_SERVER_ADDRESS="localhost:50051"

# Directorio con los .pem (certificados)
CERT_DIR="certs/dev"
SERVER_CERT="${CERT_DIR}/remote_process-cert.pem"
SERVER_KEY="${CERT_DIR}/remote_process-key.pem"
CA_CERT="${CERT_DIR}/ca-cert.pem"

# Secreto para JWT (ajusta según tu proyecto)
JWT_SECRET="YOUR_JWT_SECRET_KEY"

# Opciones de compilación/ejecución
BIN_DIR="bin"
SERVER_PID=""
TMP_OUTPUT="/tmp/grpc_test_output"
TEST_RESULTS=()

MAX_RETRIES=5
RETRY_DELAY=3

# ----------------------
# FUNCIONES AUXILIARES
# ----------------------

header() {
    echo -e "\n\033[1;34m$1\033[0m"
}

success() {
    echo -e "\033[1;32m✓ $1\033[0m"
}

error() {
    echo -e "\033[1;31m✗ $1\033[0m"
}

# Generar un token JWT simple (sin librerías externas, usando openssl)
gen_jwt() {
    local header=$(echo -n '{"alg":"HS256","typ":"JWT"}' | base64 -w 0 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
    local payload=$(echo -n '{"sub":"test-user","role":"admin","exp":4683864000}' | base64 -w 0 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
    local signature=$(echo -n "${header}.${payload}" | openssl dgst -binary -sha256 -hmac "${JWT_SECRET}" | base64 -w 0 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
    echo "${header}.${payload}.${signature}"
}

# Compilar tu binario Go (ajusta la ruta a tu main.go)
build_server() {
    header "Compilando servidor (remote_process)..."
    mkdir -p "${BIN_DIR}"
    go build -o "${BIN_DIR}/remote_process" remote_process/cmd/main.go
}

# Iniciar el servidor en background
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
    echo "PID: $SERVER_PID (logs en: ${BIN_DIR}/server.log)"
}

# Verificar salud vía gRPC Health Checking (opcional, si tu servidor implementa Health/Check)
check_health() {
    local jwt_token="$1"
    header "Comprobando la salud del servidor..."

    for ((i=1; i<=MAX_RETRIES; i++)); do
        if grpcurl -d '{}' \
            -H "authorization: ${jwt_token}" \
            -cacert "${CA_CERT}" \
            -cert "${CERT_DIR}/worker-cert.pem" \
            -key "${CERT_DIR}/worker-key.pem" \
            "${GRPC_SERVER_ADDRESS}" \
            grpc.health.v1.Health/Check >/dev/null 2>&1; then
            success "Servidor responde OK (salud verificada)."
            return 0
        else
            echo "Intento $i/$MAX_RETRIES: Servidor no responde, reintentando..."
            sleep "${RETRY_DELAY}"
        fi
    done

    error "Servidor no disponible tras varios intentos."
    exit 1
}

# ----------------------------------
# Pruebas con ExecuteCommand y jc
# ----------------------------------
run_execute_cmd_tests() {
    local jwt_token="$(gen_jwt)"
    header "Pruebas con ExecuteCommand (+ jc)"

    # Amplia BATERÍA de ejemplos
    # - "classic" => se hace "comando -> stdout -> jc --parser"
    # - "magic"   => se hace "jc comando..." y se combinan exit codes
    # - Errores en comando o en jc
    # - Etc.
    declare -a REQUESTS=(
        # 1) Modo "classic": ls -l / + parser --ls
        '{
          "command": ["ls", "-l", "."],
          "working_directory": "/etc",
          "analysis_options": {
            "parse_mode": "classic",
            "arguments": ["--ls"]
          }
        }'

        # 2) Modo "magic": ifconfig eth0
        '{
          "command": [],
          "analysis_options": {
            "parse_mode": "magic",
            "arguments": ["ifconfig", "eth0"]
          }
        }'

        # 3) Sin parseo (parse_mode vacío), por ej "uname -a"
        '{
          "command": ["uname", "-a"]
        }'

        # 4) Modo "magic" con error en el comando => "ls /carpeta_inexistente"
        '{
          "command": [],
          "analysis_options": {
            "parse_mode": "magic",
            "arguments": ["ls", "/carpeta_inexistente_99999"]
          }
        }'

        # 5) Modo "classic" => df -h con parser --df
        '{
          "command": ["df", "-h"],
          "analysis_options": {
            "parse_mode": "classic",
            "arguments": ["--df"]
          }
        }'

        # 6) Modo "classic" con parser inexistente => error en jc
        '{
          "command": ["ls", "-l"],
          "working_directory": "/tmp",
          "analysis_options": {
            "parse_mode": "classic",
            "arguments": ["--foobarParserInexistente"]
          }
        }'

        # 7) Modo "magic", comando inexistente => "myfakecmd"
        '{
          "command": [],
          "analysis_options": {
            "parse_mode": "magic",
            "arguments": ["myfakecmd", "-v"]
          }
        }'

        # 8) Modo "magic", ping google.com con parse => "jc ping -c 2 google.com" (hay parser ping en jc? Depende)
        # Si no existe parser ping en jc, verás "parse error" o "raw" => hazlo para ver comportamiento
        '{
          "command": [],
          "analysis_options": {
            "parse_mode": "magic",
            "arguments": ["ping", "-c", "2", "google.com"]
          }
        }'

        # 9) Modo "classic", cat /etc/passwd con parser "cat"? No hay parser cat en jc (ver error)
        '{
          "command": ["cat", "/etc/passwd"],
          "analysis_options": {
            "parse_mode": "classic",
            "arguments": ["--cat"]
          }
        }'

        # 10) Comando con variables de entorno
        '{
          "command": ["bash", "-c", "echo Hola $NAME"],
          "environment": { "NAME": "Mundo" },
          "analysis_options": {
            "parse_mode": "classic",
            "arguments": ["--ls"]  // absurdo parse, pero para test
          }
        }'
    )

    for REQ in "${REQUESTS[@]}"; do
        # Identificar parse_mode y "arguments" para nombrar el test
        local pm=$(echo "$REQ" | jq -r '.analysis_options.parse_mode?' 2>/dev/null)
        [ "$pm" == "null" ] && pm="no_parse"
        local args=$(echo "$REQ" | jq -r '.analysis_options.arguments?|join(" ")' 2>/dev/null)
        [ "$args" == "null" ] && args=""
        local test_name="(${pm})[${args}]_$(date +%s%N | cut -b1-13)"

        echo -e "\n\033[1;36mTEST: $test_name\033[0m"
        echo -e "Petición (JSON): \033[33m$REQ\033[0m\n"

        grpcurl -d "$REQ" \
            -H "authorization: bearer ${jwt_token}" \
            -cacert "${CA_CERT}" \
            -cert "${CERT_DIR}/worker-cert.pem" \
            -key "${CERT_DIR}/worker-key.pem" \
            "${GRPC_SERVER_ADDRESS}" \
            remote_process.RemoteProcessService/ExecuteCommand > "${TMP_OUTPUT}" 2>&1

        echo -e "\033[1;36mRespuesta:\033[0m"
        cat "${TMP_OUTPUT}" | jq

        # Si aparece "code" a nivel raíz, probablemente sea un error gRPC
        if grep -q "\"code\"" "${TMP_OUTPUT}"; then
            error "Fallo gRPC en ${test_name}"
            echo -e "\033[31m$(cat "${TMP_OUTPUT}")\033[0m"
            TEST_RESULTS+=("FAIL: ${test_name}")
        else
            # Extraer campo success y exitCode de la respuesta
            local sc=$(jq -r '.success' "${TMP_OUTPUT}")
            local ec=$(jq -r '.exitCode' "${TMP_OUTPUT}")
            local stderr_out=$(jq -r '.errorOutput' "${TMP_OUTPUT}")

            if [ "${sc}" == "true" ]; then
                success "Test ${test_name} => success=true, exitCode=${ec}"
                echo -e "\033[32mstderr:\n$stderr_out\033[0m"
                TEST_RESULTS+=("PASS: ${test_name}")
            else
                error "Test ${test_name} => success=false, exitCode=${ec}"
                echo -e "stderr => \033[31m$stderr_out\033[0m"
                TEST_RESULTS+=("FAIL: ${test_name}")
            fi
        fi
    done
}

# Reporte final
generate_report() {
    header "Resultados de las pruebas ExecuteCommand (+ jc):"
    for result in "${TEST_RESULTS[@]}"; do
        if [[ "$result" == PASS* ]]; then
            echo -e "\033[32m${result}\033[0m"
        else
            echo -e "\033[31m${result}\033[0m"
        fi
    done
}

# Limpieza (detener server)
cleanup() {
    header "Deteniendo servidor..."
    if [[ -n "$SERVER_PID" ]]; then
        kill "$SERVER_PID" 2>/dev/null || true
    fi
    rm -f "${TMP_OUTPUT}"
}

trap cleanup EXIT

# ----------------------------------
# FLUJO PRINCIPAL
# ----------------------------------
main() {
    # 1) Compilar
    build_server

    # 2) Iniciar servidor
    start_server

    # 3) Generar token y comprobar salud (opcional)
    local jwt_token="$(gen_jwt)"
    check_health "$jwt_token"

    # 4) Ejecutar pruebas del nuevo método
    run_execute_cmd_tests

    # 5) Generar reporte
    generate_report
}

main
