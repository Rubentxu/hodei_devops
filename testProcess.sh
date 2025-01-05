#!/bin/bash

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
         }'

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
           "check_interval": 5
         }'

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
         }'

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
         }'

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
         }'

# Health Check for Test 4
echo "===================="
echo "Executing Health Check for Test 4"
echo "===================="
timeout 4s curl -X POST http://localhost:8080/health \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "process_id": "print-env",
           "check_interval": 5
         }'

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
         }'

# Test 6: Long-running process 1
echo "===================="
echo "Executing Test 6: Long-running process 1"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["bash", "-c", "while true; do echo 'Long process 1...'; sleep 5; done"]' &

# Stop Test 6
echo "===================="
echo "Stopping Test 6: Long-running process 1"
echo "===================="
curl -X POST http://localhost:8080/stop \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "process_id": "long-process-1"
         }'

# Test 7: Long-running process 2
echo "===================="
echo "Executing Test 7: Long-running process 2"
echo "===================="
curl -X POST http://localhost:8080/run \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "command": ["bash", "-c", "while true; do echo 'Long process 2...'; sleep 5; done"]' &

# Stop Test 7
echo "===================="
echo "Stopping Test 7: Long-running process 2"
echo "===================="
curl -X POST http://localhost:8080/stop \
     -H "Content-Type: application/json" \
     -d '{
           "remote_process_server_address": "localhost:50051",
           "process_id": "long-process-2"
         }'