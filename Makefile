TEST_SCRIPT=./testProcess.sh
PROTOC = protoc
PROTO_DIR = protos/remote_process
PROTO_FILE = $(PROTO_DIR)/remote_process.proto
GO_OUT = .

# Directorios para certificados
CERT_DIR := certs
DEV_CERT_DIR := $(CERT_DIR)/dev
PROD_CERT_DIR := $(CERT_DIR)/prod

# Nombres de archivos de certificados
CA_KEY := ca-key.pem
CA_CERT := ca-cert.pem
SERVER_KEY := remote_process-key.pem
SERVER_CERT := remote_process-cert.pem
CLIENT_KEY := worker-key.pem
CLIENT_CERT := worker-cert.pem

# Modificar las variables JWT
JWT_SECRET ?= "test_secret_key_for_development_1234567890"

# Generar el token JWT (formato: header.payload.signature)
JWT_HEADER_B64 = $(shell echo -n '{"alg":"HS256","typ":"JWT"}' | base64 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
JWT_PAYLOAD_B64 = $(shell echo -n '{"sub":"test-user","role":"admin","exp":4683864000}' | base64 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
JWT_SIGNATURE = $(shell echo -n "$(JWT_HEADER_B64).$(JWT_PAYLOAD_B64)" | openssl dgst -binary -sha256 -hmac "$(JWT_SECRET)" | base64 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
JWT_TOKEN = $(JWT_HEADER_B64).$(JWT_PAYLOAD_B64).$(JWT_SIGNATURE)

show-jwt:
	@echo "JWT Header: $(JWT_HEADER_B64)"
	@echo "JWT Payload	: $(JWT_PAYLOAD_B64)"
	@echo "JWT Signature: $(JWT_SIGNATURE)"
	@echo "JWT	: $(JWT_TOKEN)"

# Agregar variable para el socket de Docker
DOCKER_SOCKET ?= /var/run/docker.sock

# Swagger configuration
SWAGGER_UI_VERSION ?= v4.15.5
SWAGGER_UI_DIR = worker/swagger-ui
API_DOCS_DIR = worker/api

.PHONY: proto
proto:
	@echo "🔨 Generando código desde archivos proto..."
	mkdir -p $(PROTO_DIR)/$(GO_OUT)
	$(PROTOC) --go_out=$(GO_OUT) --go_opt=paths=source_relative \
	          --go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
	          $(PROTO_FILE)

.PHONY: test
test: proto build run-remote_process run-worker
	@sleep 2 # Espera a que el servidor y el workere se inicien
	@echo "🧪 Ejecutando pruebas..."
	@echo "🧪 Ejecutando pruebas shell..."
	@bash $(TEST_SCRIPT)
	

.PHONY: test-go
test-go: stop-remote_process stop-worker clean build run-remote_process run-worker
	@sleep 5 # Increase sleep time to ensure the remote_process starts
	@echo "🧪 Ejecutando pruebas Go..."
	@JWT_SECRET="$(JWT_SECRET)" JWT_TOKEN="$(JWT_TOKEN)" go test -v ./tests/...
	@echo "✅ Pruebas Go completadas."
	@$(MAKE) stop-remote_process
	@$(MAKE) stop-worker

.PHONY: test-integration
test-integration:
	@echo "🧪 Ejecutando tests de integración..."
	cd tests && go test -v ./integration/...

.PHONY: test-all
test-all: test test-integration

.PHONY: clean
clean: stop-remote_process stop-worker
	@echo "🧹 Limpiando binarios..."
	rm -f bin/remote_process bin/worker

.PHONY: build
build:
	@echo "🏗️  Construyendo binarios..."
	go build -o bin/remote_process remote_process/cmd/main.go
	go build -o bin/worker worker/cmd/main.go

.PHONY: certs-dirs
certs-dirs:
	@echo "🔐 Creating certificate directories..."
	@mkdir -p $(DEV_CERT_DIR) $(PROD_CERT_DIR)


.PHONY: certs-dev
certs-dev: certs-dirs
	@if [ ! -f "$(DEV_CERT_DIR)/$(CA_CERT)" ] || [ ! -f "$(DEV_CERT_DIR)/$(SERVER_CERT)" ] || [ ! -f "$(DEV_CERT_DIR)/$(CLIENT_CERT)" ]; then \
		echo "🔐 Generating development certificates..."; \
		openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
			-keyout $(DEV_CERT_DIR)/$(CA_KEY) \
			-out $(DEV_CERT_DIR)/$(CA_CERT) \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=DevCA" \
			-addext "subjectAltName = DNS:localhost,DNS:remote-process,DNS:worker"; \
		openssl genrsa -out $(DEV_CERT_DIR)/$(SERVER_KEY) 4096; \
		openssl req -new -key $(DEV_CERT_DIR)/$(SERVER_KEY) \
			-out $(DEV_CERT_DIR)/remote_process.csr \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=remote-process"; \
		echo "subjectAltName=DNS:localhost,DNS:remote-process,DNS:worker" > $(DEV_CERT_DIR)/extfile.cnf; \
		openssl x509 -req \
			-in $(DEV_CERT_DIR)/remote_process.csr \
			-CA $(DEV_CERT_DIR)/$(CA_CERT) \
			-CAkey $(DEV_CERT_DIR)/$(CA_KEY) \
			-CAcreateserial \
			-out $(DEV_CERT_DIR)/$(SERVER_CERT) \
			-days 365 \
			-extfile $(DEV_CERT_DIR)/extfile.cnf; \
		openssl genrsa -out $(DEV_CERT_DIR)/$(CLIENT_KEY) 4096; \
		openssl req -new -key $(DEV_CERT_DIR)/$(CLIENT_KEY) \
			-out $(DEV_CERT_DIR)/worker.csr \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=worker"; \
		openssl x509 -req \
			-in $(DEV_CERT_DIR)/worker.csr \
			-CA $(DEV_CERT_DIR)/$(CA_CERT) \
			-CAkey $(DEV_CERT_DIR)/$(CA_KEY) \
			-CAcreateserial \
			-out $(DEV_CERT_DIR)/$(CLIENT_CERT) \
			-days 365; \
		rm $(DEV_CERT_DIR)/*.csr $(DEV_CERT_DIR)/*.srl $(DEV_CERT_DIR)/extfile.cnf; \
		chmod 600 $(DEV_CERT_DIR)/*.pem; \
		echo "✅ Development certificates generated in $(DEV_CERT_DIR)"; \
	else \
		echo "✅ Development certificates already exist in $(DEV_CERT_DIR)"; \
	fi

.PHONY: k8s-secrets
k8s-secrets: certs-dev
	@echo "🔐 Creating Kubernetes TLS secrets..."
	@kubectl create secret tls grpc-tls-certs \
		--cert=$(DEV_CERT_DIR)/$(SERVER_CERT) \
		--key=$(DEV_CERT_DIR)/$(SERVER_KEY) \
		--dry-run=worker -o yaml > k8s/tls-secret.yaml
	@kubectl create configmap grpc-ca-cert \
		--from-file=ca.crt=$(DEV_CERT_DIR)/$(CA_CERT) \
		--dry-run=worker -o yaml > k8s/ca-configmap.yaml
	@echo "✅ Kubernetes secrets generated in k8s/"

.PHONY: run-remote_process
run-remote_process: stop-remote_process build
	@echo "🚀 Starting remote_process with TLS and JWT in development mode..."
	@SERVER_CERT_PATH=$(DEV_CERT_DIR)/$(SERVER_CERT) \
	SERVER_KEY_PATH=$(DEV_CERT_DIR)/$(SERVER_KEY) \
	CA_CERT_PATH=$(DEV_CERT_DIR)/$(CA_CERT) \
	APPLICATION_PORT=50051 \
	JWT_SECRET="$(JWT_SECRET)" \
	ENV=development \
	./bin/remote_process > ./bin/remote_process.log 2>&1 & echo $$! > ./bin/remote_process.pid


.PHONY: run-worker
run-worker: stop-worker build
	@echo "🚀 Starting worker with TLS and JWT in development mode..."
	@CLIENT_CERT_PATH=$(DEV_CERT_DIR)/$(CLIENT_CERT) \
	CLIENT_KEY_PATH=$(DEV_CERT_DIR)/$(CLIENT_KEY) \
	CA_CERT_PATH=$(DEV_CERT_DIR)/$(CA_CERT) \
	JWT_TOKEN="$(JWT_TOKEN)" \
	./bin/worker serve --dir="./test_pb_data" > ./bin/worker.log 2>&1 & echo $$! > ./bin/worker.pid

.PHONY: test-tls
test-tls: certs-dev
	@echo "🧪 Testing TLS configuration..."
	@echo "Testing remote_process certificate:"
	@openssl x509 -in $(DEV_CERT_DIR)/$(SERVER_CERT) -text -noout | grep "Subject:"
	@echo "Testing worker certificate:"
	@openssl x509 -in $(DEV_CERT_DIR)/$(CLIENT_CERT) -text -noout | grep "Subject:"
	@echo "Verifying remote_process certificate against CA:"
	@openssl verify -CAfile $(DEV_CERT_DIR)/$(CA_CERT) $(DEV_CERT_DIR)/$(SERVER_CERT)
	@echo "Verifying worker certificate against CA:"
	@openssl verify -CAfile $(DEV_CERT_DIR)/$(CA_CERT) $(DEV_CERT_DIR)/$(CLIENT_CERT)

.PHONY: clean-certs
clean-certs:
	@echo "🧹 Cleaning certificates..."
	@rm -rf $(CERT_DIR)

.PHONY: docker-compose-dev
docker-compose-dev: certs-dev
	@echo "🐳 Starting services with TLS in development mode..."
	@CERT_DIR=$(DEV_CERT_DIR) docker compose up --build

# Target para generar certificados en producción (requiere variables de entorno o vault)
.PHONY: certs-prod
certs-prod:
	@echo "⚠️ Production certificates should be managed by a certificate authority"
	@echo "Please configure your production certificates manually or through your CI/CD pipeline"

# Ayuda específica para certificados
.PHONY: help-certs
help-certs:
	@echo "Certificate Management Commands:"
	@echo "  make certs-dev        - Generate development certificates"
	@echo "  make certs-prod       - Instructions for production certificates"
	@echo "  make k8s-secrets      - Generate Kubernetes TLS secrets"
	@echo "  make test-tls         - Test TLS configuration"
	@echo "  make clean-certs      - Remove all certificates"
	@echo "Development Commands:"
	@echo "  make run-remote_process   - Run remote_process with TLS in development"
	@echo "  make run-remote_process   - Run remote_process with TLS in development"
	@echo "  make run-worker   - Run worker with TLS in development"
	@echo "  make docker-compose-dev - Run all services with TLS in development"

.PHONY: install-swagger
install-swagger:
	@echo "📚 Installing swagger..."
	@go install github.com/go-swagger/go-swagger/cmd/swagger@latest

.PHONY: swagger-ui
swagger-ui:
	@echo "📚 Setting up Swagger UI..."
	@mkdir -p $(SWAGGER_UI_DIR)
	@rm -rf $(SWAGGER_UI_DIR)/*
	@curl -L -o $(SWAGGER_UI_DIR)/swagger-ui.tar.gz https://github.com/swagger-api/swagger-ui/archive/$(SWAGGER_UI_VERSION).tar.gz
	@tar -xzf $(SWAGGER_UI_DIR)/swagger-ui.tar.gz -C $(SWAGGER_UI_DIR) --strip-components=2 swagger-ui-$(SWAGGER_UI_VERSION:v%=%)/dist
	@rm $(SWAGGER_UI_DIR)/swagger-ui.tar.gz
	@sed -i 's|https://petstore.swagger.io/v2/swagger.json|/swagger.json|g' $(SWAGGER_UI_DIR)/swagger-initializer.js
	@echo "✅ Swagger UI setup complete at $(SWAGGER_UI_DIR)"

.PHONY: swagger-gen
swagger-gen: install-swagger
	@echo "📚 Generating Swagger documentation..."
	@mkdir -p $(API_DOCS_DIR)
	@swagger generate spec -o $(API_DOCS_DIR)/swagger.json --scan-models
	@echo "✅ Swagger spec generated at $(API_DOCS_DIR)/swagger.json"

.PHONY: swagger-serve
swagger-serve: swagger-gen
	@echo "📚 Serving Swagger documentation..."
	@swagger serve -F=swagger $(API_DOCS_DIR)/swagger.json

.PHONY: swagger
swagger: swagger-gen swagger-ui
	@echo "✅ Swagger setup complete"

.PHONY: stop-remote_process
stop-remote_process:
	@echo "🛑 Deteniendo servidor..."
	@kill `cat ./bin/remote_process.pid` || true
	@rm -f ./bin/remote_process.pid

.PHONY: stop-worker
stop-worker:
	@echo "🛑 Deteniendo worker..."
	@kill `cat ./bin/worker.pid` || true
	@rm -f ./bin/worker.pid

# Agregar nuevos targets para tests en Docker
.PHONY: test-docker
test-docker: check-docker certs-dev
	@echo "🐳 Running tests in Docker containers..."
	@if [ ! -w "$(DOCKER_SOCKET)" ]; then \
		echo "🔑 Requesting sudo access for Docker..."; \
		sudo chmod 666 $(DOCKER_SOCKET); \
	fi
	@JWT_SECRET="$(JWT_SECRET)" JWT_TOKEN="$(JWT_TOKEN)" \
	docker compose -f docker-compose.test.yml up \
		--build \
		--abort-on-container-exit \
		--exit-code-from tests

.PHONY: test-docker-clean
test-docker-clean:
	@echo "🧹 Cleaning up Docker test containers..."
	@#DOCKER_HOST=unix://$(DOCKER_SOCKET) \
	docker compose -f docker-compose.test.yml down -v --remove-orphans

.PHONY: check-docker
check-docker:
	@echo "🔍 Checking Docker daemon..."
	@if ! docker info > /dev/null 2>&1; then \
		if [ ! -w "$(DOCKER_SOCKET)" ]; then \
			echo "🔑 Docker socket requires permissions. Requesting sudo access..."; \
			sudo chmod 666 $(DOCKER_SOCKET); \
		else \
			echo "❌ Docker is not running."; \
			echo "Please start Docker with: sudo systemctl start docker"; \
			exit 1; \
		fi \
	fi
