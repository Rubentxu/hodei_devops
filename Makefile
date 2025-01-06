TEST_SCRIPT=./testProcess.sh
PROTOC = protoc
PROTO_DIR = internal/adapters/grpc/protos/remote_process
PROTO_FILE = $(PROTO_DIR)/remote_process.proto
GO_OUT = .

.PHONY: proto
proto:
	@echo "🔨 Generando código desde archivos proto..."
	mkdir -p $(PROTO_DIR)/$(GO_OUT)
	$(PROTOC) --go_out=$(GO_OUT) --go_opt=paths=source_relative \
	          --go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
	          $(PROTO_FILE)

.PHONY: test
test: proto build run-server run-client
	@sleep 2 # Espera a que el servidor y el cliente se inicien
	@echo "🧪 Ejecutando pruebas..."
	@echo "🧪 Ejecutando pruebas shell..."
	@bash $(TEST_SCRIPT)
	@echo "🧪 Ejecutando pruebas Go..."
	@go run cmd/test/main.go
	@echo "✅ Pruebas completadas."
	@$(MAKE) stop-server
	@$(MAKE) stop-client

.PHONY: test-go
test-go: clean build run-server run-client
	@sleep 2
	@echo "🧪 Ejecutando pruebas Go..."
	@go run cmd/test/main.go
	@echo "✅ Pruebas Go completadas."
	@$(MAKE) stop-server
	@$(MAKE) stop-client

.PHONY: test-integration
test-integration:
	@echo "🧪 Ejecutando tests de integración..."
	cd tests && go test -v ./integration/...

.PHONY: test-all
test-all: test test-integration

.PHONY: clean
clean: stop-server stop-client
	@echo "🧹 Limpiando binarios..."
	rm -f bin/server bin/client
#	@echo "🧹 Limpiando archivos generados..."
#	find $(PROTO_DIR) -name "*.pb.go" -delete

.PHONY: build
build:
	@echo "🏗️  Construyendo binarios..."
	go build -o bin/server cmd/server/main.go
	go build -o bin/client cmd/client/main.go

.PHONY: run-server
run-server:
	@echo "Stopping any existing server on port 50051..."
	@fuser -k 50051/tcp || true
	@echo "🚀 Iniciando servidor..."
	@nohup ./bin/server > ./bin/server.log 2>&1 & echo $$! > ./bin/server.pid

.PHONY: run-client
run-client:
	@echo "🚀 Iniciando cliente..."
	@nohup ./bin/client > ./bin/client.log 2>&1 & echo $$! > ./bin/client.pid

.PHONY: stop-server
stop-server:
	@echo "🛑 Deteniendo servidor..."
	@kill `cat ./bin/server.pid` || true
	@rm -f ./bin/server.pid

.PHONY: stop-client
stop-client:
	@echo "🛑 Deteniendo cliente..."
	@kill `cat ./bin/client.pid` || true
	@rm -f ./bin/client.pid

.PHONY: kill-8080
kill-8080:
	@echo "🔍 Encontrando y matando el proceso que usa el puerto 8080..."
	@sudo kill -9 $$(sudo lsof -t -i :8080) || true