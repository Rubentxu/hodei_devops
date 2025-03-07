TEST_SCRIPT=./testProcess.sh
PROTOC = protoc
PROTO_DIR = protos/remote_worker
PROTO_FILE = $(PROTO_DIR)/remote_worker.proto
GO_OUT = .

# Directorios para certificados
CERT_DIR := hodei-chart/certs
DEV_CERT_DIR := $(CERT_DIR)/dev
PROD_CERT_DIR := $(CERT_DIR)/prod

# Nombres de archivos de certificados
CA_KEY := ca-key.pem
CA_CERT := ca-cert.pem
SERVER_KEY := remote_worker-key.pem
SERVER_CERT := remote_worker-cert.pem
CLIENT_KEY := worker-client-key.pem
CLIENT_CERT := worker-client-cert.pem

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
SWAGGER_UI_DIR = hodei-app/swagger-ui
API_DOCS_DIR = hodei-app/api

# Detectar Sistema Operativo
ifeq ($(OS),Windows_NT)
    DETECTED_OS := Windows
    # Ajuste de comandos para Windows
    RM := del /F /Q
    MKDIR := mkdir
    RMDIR := rmdir /S /Q
    CP := copy
    SEP := \\
else
    DETECTED_OS := $(shell uname -s)
    # Comandos para Linux/Mac
    RM := rm -f
    MKDIR := mkdir -p
    RMDIR := rm -rf
    CP := cp
    SEP := /
endif

# Función para normalizar rutas según el SO
define normalize_path
$(subst /,$(SEP),$1)
endef

.PHONY: proto
proto:
	@echo "🔨 Generando código desde archivos proto..."
	mkdir -p $(PROTO_DIR)/$(GO_OUT)
	$(PROTOC) --go_out=$(GO_OUT) --go_opt=paths=source_relative \
	          --go-grpc_out=$(GO_OUT) --go-grpc_opt=paths=source_relative \
	          $(PROTO_FILE)

.PHONY: test
test: proto build run-remote_worker run-hodei-app
	@sleep 2 # Espera a que el servidor y el hodei-appe se inicien
	@echo "🧪 Ejecutando pruebas..."
	@echo "🧪 Ejecutando pruebas shell..."
	@bash $(TEST_SCRIPT)
	

.PHONY: test-go
test-go: stop-remote_worker stop-hodei-app clean build run-remote_worker run-hodei-app
	@sleep 5 # Increase sleep time to ensure the remote_worker starts
	@echo "🧪 Ejecutando pruebas Go..."
	@JWT_SECRET="$(JWT_SECRET)" JWT_TOKEN="$(JWT_TOKEN)" go test -v ./tests/...
	@echo "✅ Pruebas Go completadas."
	@$(MAKE) stop-remote_worker
	@$(MAKE) stop-hodei-app

.PHONY: test-integration
test-integration:
	@echo "🧪 Ejecutando tests de integración..."
	cd tests && go test -v ./integration/...

.PHONY: test-all
test-all: test test-integration

.PHONY: clean
clean: stop-remote_worker stop-hodei-app
	@echo "🧹 Limpiando binarios..."
ifeq ($(DETECTED_OS),Windows)
	@$(RM) bin$(SEP)remote_worker.exe 2>NUL || true
	@$(RM) bin$(SEP)hodei-app.exe 2>NUL || true
	@$(RM) bin$(SEP)archiva-go.exe 2>NUL || true
else
	@$(RM) bin/remote_worker bin/hodei-app bin/archiva-go 2>/dev/null || true
endif

.PHONY: build
build:
	@echo "🏗️  Construyendo binarios..."
	go build -o bin/remote_worker remote_worker/cmd/main.go
	go build -o bin/hodei-app hodei-app/cmd/main.go
	go build -o bin/archiva-go archiva_go/cmd/main.go

# Directorios para certificados - Cross-platform
.PHONY: certs-dirs
certs-dirs:
	@echo "🔐 Creating certificate directories..."
ifeq ($(DETECTED_OS),Windows)
	@if not exist $(call normalize_path,$(DEV_CERT_DIR)) $(MKDIR) $(call normalize_path,$(DEV_CERT_DIR))
	@if not exist $(call normalize_path,$(PROD_CERT_DIR)) $(MKDIR) $(call normalize_path,$(PROD_CERT_DIR))
else
	@$(MKDIR) $(DEV_CERT_DIR) $(PROD_CERT_DIR)
endif

# Certificados - Compatible con Windows/Linux
.PHONY: certs-dev
certs-dev: certs-dirs
ifeq ($(DETECTED_OS),Windows)
	@if not exist "$(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_CERT))" (echo "🔐 Generating development certificates..." && \
		openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
			-keyout $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_KEY)) \
			-out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_CERT)) \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=DevCA" \
			-addext "subjectAltName = DNS:localhost,DNS:remote-process,DNS:worker" && \
		openssl genrsa -out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(SERVER_KEY)) 4096 && \
		openssl req -new -key $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(SERVER_KEY)) \
			-out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)remote_worker.csr) \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=remote-process" && \
		echo "subjectAltName=DNS:localhost,DNS:remote-process,DNS:worker" > $(call normalize_path,$(DEV_CERT_DIR)$(SEP)extfile.cnf) && \
		openssl x509 -req \
			-in $(call normalize_path,$(DEV_CERT_DIR)$(SEP)remote_worker.csr) \
			-CA $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_CERT)) \
			-CAkey $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_KEY)) \
			-CAcreateserial \
			-out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(SERVER_CERT)) \
			-days 365 \
			-extfile $(call normalize_path,$(DEV_CERT_DIR)$(SEP)extfile.cnf) && \
		openssl genrsa -out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CLIENT_KEY)) 4096 && \
		openssl req -new -key $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CLIENT_KEY)) \
			-out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)worker-client.csr) \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=worker" && \
		openssl x509 -req \
			-in $(call normalize_path,$(DEV_CERT_DIR)$(SEP)worker-client.csr) \
			-CA $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_CERT)) \
			-CAkey $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_KEY)) \
			-CAcreateserial \
			-out $(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CLIENT_CERT)) \
			-days 365 && \
		del $(call normalize_path,$(DEV_CERT_DIR)$(SEP)*.csr) $(call normalize_path,$(DEV_CERT_DIR)$(SEP)*.srl) $(call normalize_path,$(DEV_CERT_DIR)$(SEP)extfile.cnf) && \
		echo "✅ Development certificates generated in $(DEV_CERT_DIR)") \
	else \
		echo "✅ Development certificates already exist in $(DEV_CERT_DIR)" \
	)
else
	@if [ ! -f "$(DEV_CERT_DIR)/$(CA_CERT)" ] || [ ! -f "$(DEV_CERT_DIR)/$(SERVER_CERT)" ] || [ ! -f "$(DEV_CERT_DIR)/$(CLIENT_CERT)" ]; then \
		echo "🔐 Generating development certificates..."; \
		openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
			-keyout $(DEV_CERT_DIR)/$(CA_KEY) \
			-out $(DEV_CERT_DIR)/$(CA_CERT) \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=DevCA" \
			-addext "subjectAltName = DNS:localhost,DNS:remote-process,DNS:worker"; \
		openssl genrsa -out $(DEV_CERT_DIR)/$(SERVER_KEY) 4096; \
		openssl req -new -key $(DEV_CERT_DIR)/$(SERVER_KEY) \
			-out $(DEV_CERT_DIR)/remote_worker.csr \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=remote-process"; \
		echo "subjectAltName=DNS:localhost,DNS:remote-process,DNS:worker" > $(DEV_CERT_DIR)/extfile.cnf; \
		openssl x509 -req \
			-in $(DEV_CERT_DIR)/remote_worker.csr \
			-CA $(DEV_CERT_DIR)/$(CA_CERT) \
			-CAkey $(DEV_CERT_DIR)/$(CA_KEY) \
			-CAcreateserial \
			-out $(DEV_CERT_DIR)/$(SERVER_CERT) \
			-days 365 \
			-extfile $(DEV_CERT_DIR)/extfile.cnf; \
		openssl genrsa -out $(DEV_CERT_DIR)/$(CLIENT_KEY) 4096; \
		openssl req -new -key $(DEV_CERT_DIR)/$(CLIENT_KEY) \
			-out $(DEV_CERT_DIR)/worker-client.csr \
			-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=worker"; \
		openssl x509 -req \
			-in $(DEV_CERT_DIR)/worker-client.csr \
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
endif


# Run remote_worker - Cross-platform
.PHONY: run-remote_worker
run-remote_worker: stop-remote_worker build
	@echo "🚀 Starting remote_worker with TLS and JWT in development mode..."
ifeq ($(DETECTED_OS),Windows)
	@set SERVER_CERT_PATH=$(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(SERVER_CERT))& ^
	set SERVER_KEY_PATH=$(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(SERVER_KEY))& ^
	set CA_CERT_PATH=$(call normalize_path,$(DEV_CERT_DIR)$(SEP)$(CA_CERT))& ^
	set APPLICATION_PORT=50051& ^
	set JWT_SECRET=$(JWT_SECRET)& ^
	set ENV=development& ^
	start /B cmd /C bin$(SEP)remote_worker > bin$(SEP)remote_worker.log 2>&1
	@echo %ERRORLEVEL% > bin$(SEP)remote_worker.pid
else
	@SERVER_CERT_PATH=$(DEV_CERT_DIR)/$(SERVER_CERT) \
	SERVER_KEY_PATH=$(DEV_CERT_DIR)/$(SERVER_KEY) \
	CA_CERT_PATH=$(DEV_CERT_DIR)/$(CA_CERT) \
	APPLICATION_PORT=50051 \
	JWT_SECRET="$(JWT_SECRET)" \
	ENV=development \
	./bin/remote_worker > ./bin/remote_worker.log 2>&1 & echo $$! > ./bin/remote_worker.pid
endif

# Run hodei-app - Cross-platform
.PHONY: run-hodei-app
run-hodei-app: stop-hodei-app build
	@echo "🚀 Starting hodei-app with TLS and JWT in development mode..."
ifeq ($(DETECTED_OS),Windows)
	@set JWT_TOKEN=$(JWT_TOKEN)& ^
	start /B cmd /C bin$(SEP)hodei-app serve --dir=".\test_pb_data" > bin$(SEP)hodei-app.log 2>&1
	@echo %ERRORLEVEL% > bin$(SEP)hodei-app.pid
else
	@JWT_TOKEN="$(JWT_TOKEN)" \
	DEFAULT_DOCKER_POOL=true \
	./bin/hodei-app serve --dir="./test_pb_data" > ./bin/hodei-app.log 2>&1 & echo $$! > ./bin/hodei-app.pid
endif

.PHONY: stop-hodei-app
stop-hodei-app:
	@echo "🛑 Deteniendo hodei-app..."
ifeq ($(DETECTED_OS),Windows)
	@if exist bin$(SEP)hodei-app.pid (for /f %i in (bin$(SEP)hodei-app.pid) do taskkill /F /PID %i 2>NUL || true)
	@$(RM) bin$(SEP)hodei-app.pid 2>NUL || true
else
	@if [ -f ./bin/hodei-app.pid ]; then kill -9 `cat ./bin/hodei-app.pid` 2>/dev/null || true; fi
	@$(RM) ./bin/hodei-app.pid 2>/dev/null || true
endif

.PHONY: run-archiva-go
run-archiva-go: stop-archiva-go build
	@echo "🚀 Starting archiva-go..."
	./bin/archiva-go > ./bin/archiva-go.log 2>&1 & echo $$! > ./bin/archiva-go.pid

.PHONY: test-tls
test-tls: certs-dev
	@echo "🧪 Testing TLS configuration..."
	@echo "Testing remote_worker certificate:"
	@openssl x509 -in $(DEV_CERT_DIR)/$(SERVER_CERT) -text -noout | grep "Subject:"
	@echo "Testing hodei-app certificate:"
	@openssl x509 -in $(DEV_CERT_DIR)/$(CLIENT_CERT) -text -noout | grep "Subject:"
	@echo "Verifying remote_worker certificate against CA:"
	@openssl verify -CAfile $(DEV_CERT_DIR)/$(CA_CERT) $(DEV_CERT_DIR)/$(SERVER_CERT)
	@echo "Verifying hodei-app certificate against CA:"
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
	@echo "  make run-remote_worker   - Run remote_worker with TLS in development"
	@echo "  make run-remote_worker   - Run remote_worker with TLS in development"
	@echo "  make run-hodei-app   - Run hodei-app with TLS in development"
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

.PHONY: stop-remote_worker
stop-remote_worker:
	@echo "🛑 Deteniendo servidor..."
ifeq ($(DETECTED_OS),Windows)
	@if exist bin$(SEP)remote_worker.pid (for /f %i in (bin$(SEP)remote_worker.pid) do taskkill /F /PID %i 2>NUL || true)
	@$(RM) bin$(SEP)remote_worker.pid 2>NUL || true
else
	@if [ -f ./bin/remote_worker.pid ]; then kill -9 `cat ./bin/remote_worker.pid` 2>/dev/null || true; fi
	@$(RM) ./bin/remote_worker.pid 2>/dev/null || true
endif



.PHONY: stop-archiva-go
stop-archiva-go:
	@echo "🛑 Deteniendo archiva-go..."
	@kill `cat ./bin/archiva-go.pid` || true
	@rm -f ./bin/archiva-go.pid



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

# Targets para construcción de imágenes Docker
.PHONY: docker-build-all
docker-build-all: docker-build-hodei-app docker-build-remote-process

# Añadir targets para compilar binarios específicamente para imágenes Docker
.PHONY: build-for-docker
build-for-docker: proto
	@echo "🏗️  Construyendo binarios para Docker en $(DETECTED_OS)..."
	@$(MKDIR) bin
ifeq ($(DETECTED_OS),Windows)
	@set CGO_ENABLED=0&&set GOOS=linux&&set GOARCH=amd64&&go build -o bin$(SEP)remote_worker.exe remote_worker$(SEP)cmd$(SEP)main.go
	@set CGO_ENABLED=0&&set GOOS=linux&&set GOARCH=amd64&&go build -o bin$(SEP)hodei-app.exe hodei-app$(SEP)cmd$(SEP)main.go
else
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin$(SEP)remote_worker remote_worker$(SEP)cmd$(SEP)main.go
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin$(SEP)hodei-app hodei-app$(SEP)cmd$(SEP)main.go
endif
	@echo "✅ Binarios construidos para Docker en directorio bin/"

.PHONY: docker-build-hodei-app
docker-build-hodei-app: build-for-docker
	@echo "🐳 Construyendo imagen de Docker para hodei-app..."
	docker build -t hodei/hodei-app:latest -f build/hodei-app/Dockerfile .

.PHONY: docker-build-remote-process
docker-build-remote-process: build-for-docker
	@echo "🐳 Construyendo imagen de Docker para remote_worker..."
	docker build -t hodei/remote-process-worker:latest -f build/remote_worker/Dockerfile .




# Target para test en Docker con generación de certificados
.PHONY: test-docker
test-docker: check-docker certs-dev build
	@echo "🐳 Launching hodei-app container via docker-compose..."
	@docker compose -f docker-compose.test.yml up -d --build
	@JWT_SECRET="$(JWT_SECRET)" \
    JWT_TOKEN="$(JWT_TOKEN)" \
    WS_BASE_URL="ws://hodei-app:8090" \
    API_BASE_URL="http://hodei-app:8090" \
    WORKER_IMAGE="hodei/remote-process-worker:latest" \
    go test -v ./tests/tasks_api_test.go


.PHONY: test-docker-clean
test-docker-clean:
	@echo "🧹 Cleaning up Docker test containers..."
	@docker compose -f docker-compose.test.yml down -v --remove-orphans

# Target para construir imágenes y desplegarlas en Helm
.PHONY: helm-deploy
helm-deploy: docker-build-all certs-dev setup-helm-certs
	@echo "🚢 Desplegando en Kubernetes usando Helm..."
ifeq ($(DETECTED_OS),Windows)
	helm upgrade --install hodei-chart .\hodei-chart ^
		--set hodei-app.config.grpc.jwtSecret="$(JWT_SECRET)" ^
		--set hodei-app.config.grpc.jwtToken="$(JWT_TOKEN)"
else
	helm upgrade --install hodei-chart ./hodei-chart \
		--set hodei-app.config.grpc.jwtSecret="$(JWT_SECRET)" \
		--set hodei-app.config.grpc.jwtToken="$(JWT_TOKEN)"
endif

# Target para preparar certificados para Helm (compatible con Windows/Linux)
.PHONY: setup-helm-certs
setup-helm-certs:
	@echo "🔐 Copiando certificados para Helm chart..."
ifeq ($(DETECTED_OS),Windows)
	@if not exist hodei-chart$(SEP)certs $(MKDIR) hodei-chart$(SEP)certs
	@$(CP) $(call normalize_path,$(DEV_CERT_DIR)$(SEP)*.pem) $(call normalize_path,hodei-chart$(SEP)certs$(SEP))
else
	@$(MKDIR) hodei-chart/certs
	@$(CP) $(DEV_CERT_DIR)/*.pem hodei-chart/certs/
endif

.PHONY: test-containers
test-containers: check-docker certs-dev build
	@echo "🧹 Eliminando contenedor hodei-hodei-app previo (si existe)..."
	@docker rm -f hodei-hodei-app || true
	@echo "🧪 Ejecutando tests con testcontainers-go..."
	@JWT_SECRET="$(JWT_SECRET)" \
	JWT_TOKEN="$(JWT_TOKEN)" \
	WS_BASE_URL="ws://hodei-app:8090" \
	API_BASE_URL="http://hodei-app:8090" \
	WORKER_IMAGE="hodei/remote-process-worker:latest" \
	go test -v ./tests/tasks_api_test.go
