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
test:
	@echo "🧪 Ejecutando pruebas..."
	go test ./...

.PHONY: clean
clean:
	@echo "🧹 Limpiando archivos generados..."
	find $(PROTO_DIR) -name "*.pb.go" -delete

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
	@nohup ./bin/server > server.log 2>&1 & echo $$! > server.pid

.PHONY: run-client
run-client:
	@echo "🚀 Iniciando cliente..."
	@nohup ./bin/client > client.log 2>&1 & echo $$! > client.pid

.PHONY: stop-server
stop-server:
	@echo "🛑 Deteniendo servidor..."
	@kill `cat server.pid` || true
	@rm -f server.pid

.PHONY: stop-client
stop-client:
	@echo "🛑 Deteniendo cliente..."
	@kill `cat client.pid` || true
	@rm -f client.pid

.PHONY: test
test: proto build run-server run-client
	@sleep 2 # Espera a que el servidor y el cliente se inicien
	@echo "🧪 Ejecutando pruebas..."
	@bash $(TEST_SCRIPT) # Ejecuta el script de pruebas
	@echo "✅ Pruebas completadas."
	@$(MAKE) stop-server
	@$(MAKE) stop-client

.PHONY: clean
clean:
	@echo "🧹 Limpiando binarios..."
	rm -f bin/server bin/client