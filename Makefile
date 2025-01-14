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
SERVER_KEY := server-key.pem
SERVER_CERT := server-cert.pem
CLIENT_KEY := client-key.pem
CLIENT_CERT := client-cert.pem

# Modificar las variables JWT
JWT_SECRET ?= "test_secret_key_for_development_1234567890"

# Generar el token JWT (formato: header.payload.signature)
JWT_HEADER_B64 = $(shell echo -n '{"alg":"HS256","typ":"JWT"}' | base64 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
JWT_PAYLOAD_B64 = $(shell echo -n '{"sub":"test-user","role":"admin","exp":4683864000}' | base64 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
JWT_SIGNATURE = $(shell echo -n "$(JWT_HEADER_B64).$(JWT_PAYLOAD_B64)" | openssl dgst -binary -sha256 -hmac "$(JWT_SECRET)" | base64 | tr -d '\n' | tr '/+' '_-' | tr -d '=')
JWT_TOKEN = $(JWT_HEADER_B64).$(JWT_PAYLOAD_B64).$(JWT_SIGNATURE)

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
	

.PHONY: test-go
test-go: stop-server stop-client clean build run-server run-client
	@sleep 5 # Increase sleep time to ensure the server starts
	@echo "🧪 Ejecutando pruebas Go..."
	@JWT_SECRET="$(JWT_SECRET)" JWT_TOKEN="$(JWT_TOKEN)" go run tests/main.go
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

.PHONY: build
build:
	@echo "🏗️  Construyendo binarios..."
	go build -o bin/server remote_process/cmd/main.go
	go build -o bin/client orchestrator/cmd/main.go

.PHONY: certs-dirs
certs-dirs:
	@echo "🔐 Creating certificate directories..."
	@mkdir -p $(DEV_CERT_DIR) $(PROD_CERT_DIR)

.PHONY: certs-dev
certs-dev: certs-dirs
	@echo "🔐 Generating development certificates..."
	@openssl req -x509 -newkey rsa:4096 -days 365 -nodes \
		-keyout $(DEV_CERT_DIR)/$(CA_KEY) \
		-out $(DEV_CERT_DIR)/$(CA_CERT) \
		-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=DevCA" \
		-addext "subjectAltName = DNS:localhost,DNS:remote-process"

	@openssl genrsa -out $(DEV_CERT_DIR)/$(SERVER_KEY) 4096
	@openssl req -new -key $(DEV_CERT_DIR)/$(SERVER_KEY) \
		-out $(DEV_CERT_DIR)/server.csr \
		-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=localhost"

	@echo "subjectAltName=DNS:localhost,DNS:remote-process" > $(DEV_CERT_DIR)/extfile.cnf
	@openssl x509 -req \
		-in $(DEV_CERT_DIR)/server.csr \
		-CA $(DEV_CERT_DIR)/$(CA_CERT) \
		-CAkey $(DEV_CERT_DIR)/$(CA_KEY) \
		-CAcreateserial \
		-out $(DEV_CERT_DIR)/$(SERVER_CERT) \
		-days 365 \
		-extfile $(DEV_CERT_DIR)/extfile.cnf

	@openssl genrsa -out $(DEV_CERT_DIR)/$(CLIENT_KEY) 4096
	@openssl req -new -key $(DEV_CERT_DIR)/$(CLIENT_KEY) \
		-out $(DEV_CERT_DIR)/client.csr \
		-subj "/C=ES/ST=Madrid/L=Madrid/O=DevOps/OU=Platform/CN=client"

	@openssl x509 -req \
		-in $(DEV_CERT_DIR)/client.csr \
		-CA $(DEV_CERT_DIR)/$(CA_CERT) \
		-CAkey $(DEV_CERT_DIR)/$(CA_KEY) \
		-CAcreateserial \
		-out $(DEV_CERT_DIR)/$(CLIENT_CERT) \
		-days 365

	@rm $(DEV_CERT_DIR)/*.csr $(DEV_CERT_DIR)/*.srl $(DEV_CERT_DIR)/extfile.cnf
	@chmod 600 $(DEV_CERT_DIR)/*.pem
	@echo "✅ Development certificates generated in $(DEV_CERT_DIR)"

.PHONY: k8s-secrets
k8s-secrets: certs-dev
	@echo "🔐 Creating Kubernetes TLS secrets..."
	@kubectl create secret tls grpc-tls-certs \
		--cert=$(DEV_CERT_DIR)/$(SERVER_CERT) \
		--key=$(DEV_CERT_DIR)/$(SERVER_KEY) \
		--dry-run=client -o yaml > k8s/tls-secret.yaml
	@kubectl create configmap grpc-ca-cert \
		--from-file=ca.crt=$(DEV_CERT_DIR)/$(CA_CERT) \
		--dry-run=client -o yaml > k8s/ca-configmap.yaml
	@echo "✅ Kubernetes secrets generated in k8s/"

.PHONY: run-server
run-server: build
	@echo "🚀 Starting server with TLS and JWT in development mode..."
	@SERVER_CERT_PATH=$(DEV_CERT_DIR)/$(SERVER_CERT) \
	SERVER_KEY_PATH=$(DEV_CERT_DIR)/$(SERVER_KEY) \
	CA_CERT_PATH=$(DEV_CERT_DIR)/$(CA_CERT) \
	APPLICATION_PORT=50051 \
	JWT_SECRET="$(JWT_SECRET)" \
	ENV=development \
	./bin/server > ./bin/server.log 2>&1 & echo $$! > ./bin/server.pid


.PHONY: run-client
run-client: build
	@echo "🚀 Starting client with TLS and JWT in development mode..."
	@CLIENT_CERT_PATH=$(DEV_CERT_DIR)/$(CLIENT_CERT) \
	CLIENT_KEY_PATH=$(DEV_CERT_DIR)/$(CLIENT_KEY) \
	CA_CERT_PATH=$(DEV_CERT_DIR)/$(CA_CERT) \
	JWT_TOKEN="$(JWT_TOKEN)" \
	./bin/client > ./bin/client.log 2>&1 & echo $$! > ./bin/client.pid

.PHONY: test-tls
test-tls: certs-dev
	@echo "🧪 Testing TLS configuration..."
	@echo "Testing server certificate:"
	@openssl x509 -in $(DEV_CERT_DIR)/$(SERVER_CERT) -text -noout | grep "Subject:"
	@echo "Testing client certificate:"
	@openssl x509 -in $(DEV_CERT_DIR)/$(CLIENT_CERT) -text -noout | grep "Subject:"
	@echo "Verifying server certificate against CA:"
	@openssl verify -CAfile $(DEV_CERT_DIR)/$(CA_CERT) $(DEV_CERT_DIR)/$(SERVER_CERT)
	@echo "Verifying client certificate against CA:"
	@openssl verify -CAfile $(DEV_CERT_DIR)/$(CA_CERT) $(DEV_CERT_DIR)/$(CLIENT_CERT)

.PHONY: clean-certs
clean-certs:
	@echo "🧹 Cleaning certificates..."
	@rm -rf $(CERT_DIR)

.PHONY: docker-compose-dev
docker-compose-dev: certs-dev
	@echo "🐳 Starting services with TLS in development mode..."
	@CERT_DIR=$(DEV_CERT_DIR) docker-compose up --build

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
	@echo "  make run-server   - Run server with TLS in development"
	@echo "  make run-client   - Run client with TLS in development"
	@echo "  make docker-compose-dev - Run all services with TLS in development"

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
