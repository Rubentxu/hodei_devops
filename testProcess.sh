#!/bin/bash

# Función para ejecutar health check en paralelo
run_health_check() {
    local process_id=$1
    local check_interval=$2
    echo "===================="
    echo "Executing Health Check for $process_id"
    echo "===================="
    curl -X POST http://localhost:8080/health \
         -H "Content-Type: application/json" \
         -d '{
               "remote_process_server_address": "localhost:50051",
               "process_id": "'$process_id'",
               "check_interval": '$check_interval'
             }' &
    echo $!  # Retorna el PID del proceso en background
}

# Test 1: Ping Google
echo "===================="
echo "Executing Test 1: Ping Google"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["ping", "-c", "5", "google.com"],
           "process_id": "ping-google",
           "check_interval": 5
         }' &
CURL_PID_1=$!
HEALTH_PID_1=$(run_health_check "ping-google" 5)

# Test 2: List directory contents
echo "===================="
echo "Executing Test 2: List directory contents"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["ls", "-la"],
           "process_id": "list-dir",
           "check_interval": 5,
           "working_directory": "'$HOME'"
         }' &
CURL_PID_2=$!
HEALTH_PID_2=$(run_health_check "list-dir" 5)

# Test 3: Loop with message for 10 seconds
echo "===================="
echo "Executing Test 3: Loop with message for 10 seconds"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["bash", "-c", "for i in {1..10}; do echo Looping... iteration $i; sleep 1; done"],
           "process_id": "loop-10",
           "check_interval": 5
         }' &
CURL_PID_3=$!
HEALTH_PID_3=$(run_health_check "loop-10" 5)

# Test 4: Print environment variables
echo "===================="
echo "Executing Test 4: Print environment variables"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["printenv"],
           "process_id": "env",
           "check_interval": 5,
           "env": {
             "VAR1": "value1",
             "VAR2": "value2"
           }
         }' &
CURL_PID_4=$!
HEALTH_PID_4=$(run_health_check "env" 5)

# Test 4b: Echo environment variable
echo "===================="
echo "Executing Test 4b: Echo environment variable"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["bash", "-c", "echo VAR1 is ${VAR1}"],
           "process_id": "echo-env-var",
           "check_interval": 5,
           "env": {
             "VAR1": "value1"
           }
         }' &
CURL_PID_5=$!
HEALTH_PID_5=$(run_health_check "echo-env-var" 5)

# Test 5: Echo a message
echo "===================="
echo "Executing Test 5: Echo a message"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["echo", "Hello, World!"],
           "process_id": "echo-message",
           "check_interval": 5
         }' &
CURL_PID_6=$!
HEALTH_PID_6=$(run_health_check "echo-message" 5)

# Test 6: Long-running process 1
echo "===================="
echo "Executing Test 6: Long-running process 1"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["bash", "-c", "i=1; while true; do echo \"Long process 1... iteration $i\"; i=$((i+1)); sleep 5; done"],
           "process_id": "long-process-1",
           "check_interval": 5
         }' &
CURL_PID_6=$!
HEALTH_PID_6=$(run_health_check "long-process-1" 5)

sleep 5

# Función para detener proceso y sus health checks
stop_process() {
    local process_id=$1
    local curl_pid=$2
    local health_pid=$3
    
    echo "===================="
    echo "Stopping process: $process_id"
    echo "===================="
    curl -X POST http://localhost:8080/stop \
         -H "Content-Type: application/json" \
         -d '{
               "remote_process_server_address": "localhost:50051",
               "process_id": "'$process_id'"
             }'
    
    # Matar los procesos curl en background si aún están corriendo
    kill $curl_pid 2>/dev/null || true
    kill $health_pid 2>/dev/null || true
}

# Detener todos los procesos y sus health checks
stop_process "ping-google" $CURL_PID_1 $HEALTH_PID_1
stop_process "list-dir" $CURL_PID_2 $HEALTH_PID_2
stop_process "loop-10" $CURL_PID_3 $HEALTH_PID_3
stop_process "env" $CURL_PID_4 $HEALTH_PID_4
stop_process "echo-env-var" $CURL_PID_5 $HEALTH_PID_5
stop_process "long-process-1" $CURL_PID_6 $HEALTH_PID_6