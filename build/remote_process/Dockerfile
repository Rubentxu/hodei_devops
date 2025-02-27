FROM ubuntu:24.04

# Instalar dependencias necesarias (solo las mínimas para el runtime)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    wget \
    bash && \
    rm -rf /var/lib/apt/lists/*

# Descargar y extraer grpcurl (si lo necesitas en runtime, si no, quítalo)
RUN curl -L -o grpcurl.tar.gz https://github.com/fullstorydev/grpcurl/releases/download/v1.9.2/grpcurl_1.9.2_linux_x86_64.tar.gz && \
    tar -xzf grpcurl.tar.gz -C /usr/local/bin && \
    rm grpcurl.tar.gz

WORKDIR /app

# Copia el binario precompilado y el script health check
COPY ./bin/remote_process .
COPY ./grpc_health_check.sh .

# Copia los certificados
COPY ./hodei-chart/certs/dev /certs

# Crear un usuario no privilegiado para mayor seguridad.
ARG DOCKER_UID=1001
ARG DOCKER_GID=1001

RUN groupadd -g ${DOCKER_GID} appgroup && \
    useradd -u ${DOCKER_UID} -g appgroup -m -s /bin/bash appuser

# Establecer propietario y permisos
RUN chown -R appuser:appgroup /app /certs
RUN chmod +x /app/grpc_health_check.sh
RUN chmod +x /app/remote_process
# Configurar permisos para certificados
RUN find /certs -type d -exec chmod 755 {} \; && \
    find /certs -type f -name "*.pem" -exec chmod 644 {} \; && \
    find /certs -type f -name "*key*" -exec chmod 600 {} \;

# Cambiar al usuario no privilegiado
USER appuser

# Puerto gRPC
EXPOSE 50051

# Comando para ejecutar el servidor
CMD ["./remote_process"]