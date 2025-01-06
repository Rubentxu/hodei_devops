FROM ubuntu:24.04

# Instalar Go y otras dependencias
RUN apt-get update && apt-get install -y \
    golang-go \
    git \
    make \
    netcat \
    && rm -rf /var/lib/apt/lists/*

# Configurar variables de entorno de Go
ENV GOPATH=/go
ENV PATH=$GOPATH/bin:/usr/local/go/bin:$PATH

# Crear directorio de trabajo
WORKDIR /app

# Copiar los archivos del proyecto
COPY . .

# Compilar la aplicación
RUN make build

# Exponer el puerto gRPC
EXPOSE 50051

# Comando para ejecutar el servidor con logging
CMD ["sh", "-c", "echo 'Starting server...' && ./bin/server"] 