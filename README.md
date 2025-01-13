# **Creando un MVP de Ejecución Remota de Procesos con gRPC y Arquitectura Hexagonal (Streaming Edition)**

![img.png](docs/resources/img.png)

## 🎯 **Introducción**

En este post, vamos a crear un Producto Mínimo Viable (MVP) para un sistema de **ejecución remota de procesos** utilizando **gRPC** y **Go**. Implementaremos **streaming bidireccional** para la salida del proceso y la monitorización de su estado, obteniendo así información en tiempo real.

### **¿Por qué gRPC y arquitectura hexagonal?**

- **gRPC:** Un framework moderno y de alto rendimiento para RPC, ideal para comunicación eficiente y con streaming.
- **Arquitectura Hexagonal:** También conocida como "Puertos y Adaptadores", desacopla la lógica de negocio de los detalles de implementación, como la comunicación externa o el almacenamiento de datos. Esto facilita la evolución, el mantenimiento y las pruebas del sistema.

### **Objetivo del MVP**

Nuestro MVP permitirá a los usuarios:

1. **Ejecutar comandos** en un servidor remoto.
2. **Obtener la salida del comando en tiempo real** (streaming).
3. **Detener procesos** en ejecución.
4. **Monitorizar el estado de los procesos** (streaming).

### **Pre-requisitos**

- Conocimientos básicos de Go.
- Tener instalado Go y `protoc` (el compilador de Protocol Buffers).
- Familiaridad con la línea de comandos.
- Un editor de código (recomendamos VS Code con la extensión de Go).

### **Instalación de Protocol Buffers**

Para instalar el compilador de Protocol Buffers (`protoc`), sigue las instrucciones según tu sistema operativo:

#### **Windows**
1. Visita la [página de lanzamientos de Protocol Buffers](https://github.com/protocolbuffers/protobuf/releases).
2. Descarga el archivo zip correspondiente a Windows.
3. Descomprime el archivo en una ubicación de tu elección.
4. Añade la ruta del ejecutable `protoc.exe` a la variable de entorno `PATH` para que puedas ejecutarlo desde cualquier terminal.

#### **Ubuntu Linux**
1. Abre una terminal.
2. Actualiza la lista de paquetes:
```bash
   sudo apt update
```
3. Instala el compilador de Protocol Buffers:
```bash
  sudo apt install -y protobuf-compiler
```
4. Verifica la instalación ejecutando `protoc --version` en la terminal para asegurarte de que se ha instalado correctamente.
#### **Fedora:**
1. Abre una terminal.
2. Actualiza la lista de paquetes:
```bash
  sudo dnf check-update
```
3. Instala el compilador de Protocol Buffers:
```bash
  sudo dnf install -y protobuf-compiler
```
4. Verifica la instalación ejecutando `protoc --version` en la terminal para asegurarte de que se ha instalado correctamente.

3. Instala el compilador de Protocol Buffers:
   ```bash
   sudo apt install -y protobuf-compiler
   ```
4. Verifica la instalación ejecutando `protoc --version` en la terminal para asegurarte de que se ha instalado correctamente.

---

## 🏗️ **Arquitectura del Sistema**

Seguiremos la **arquitectura hexagonal**, que divide nuestro sistema en capas. Esta arquitectura nos permite mantener una separación clara de responsabilidades y facilita el mantenimiento y la escalabilidad del sistema.

### **Capas principales de la arquitectura hexagonal:**

- **Dominio:**  
  Contiene la lógica de negocio principal de la aplicación. Aquí se definen las interfaces (puertos) que describen las operaciones que la aplicación puede realizar. Es independiente de tecnologías específicas, lo que facilita su testeo y reutilización.

- **Adaptadores:**  
  Implementan los puertos definidos en la capa de dominio. Permiten que el dominio interactúe con el mundo exterior, como servicios gRPC, bases de datos, sistemas de archivos, etc.

- **Infraestructura:**  
  Contiene el código de configuración y las utilidades que no son específicas de la lógica de negocio, como la configuración de dependencias, la inicialización de servicios, y la gestión de variables de entorno.

Cada una de estas capas se comunica con las otras a través de interfaces bien definidas, lo que permite una mayor modularidad y flexibilidad en el diseño del sistema.

### **Estructura de directorios utilizando `go.work` y arquitectura hexagonal**

`go.work` es una característica de Go que permite trabajar con múltiples módulos en un solo espacio de trabajo. Esto es especialmente útil en proyectos grandes y complejos.

#### **Ventajas de `go.work`:**
- Permite a los desarrolladores trabajar en varios módulos simultáneamente sin necesidad de publicarlos en un repositorio remoto.
- Facilita la integración continua y el despliegue.

#### Estructura de Directorios con `go.work`

```
.
├── go.work
├── cmd/
│   ├── client/
│   │   └── main.go
│   └── server/
│       └── main.go        # Ejecutable del servidor
└── internal/              # Código privado al módulo
   ├── domain/            # Lógica de negocio y interfaces
   │   ├── go.mod
   │   └── ports/         # Interfaces (puertos)
   │       └── process.go      
   └── adapters/          # Implementaciones de adaptadores
       ├── go.mod
       ├── grpc/          # Implementación gRPC
       │   ├── protos/    # Definiciones de protobuf
       │   │   └── remote_process/
       │   │   │   └── remote_process.proto
       │   ├── client/    # Cliente gRPC
       │   │   └── remote_process/
       │   │   │   └── remote_process_client.go
       │   └── server/    # Servidor gRPC
       │       └── remote_process/
       │       │   └── remote_process_server.go
       └── local/         # Implementación local para ejecución de procesos
          └── process_executor.go│   
```

#### **Descripción de los Archivos**

1. **`go.work`**: Archivo de trabajo que define los módulos que forman parte del proyecto. Este archivo permite trabajar con múltiples módulos de Go en un solo espacio de trabajo.

2. **`cmd/`**: Contiene los puntos de entrada de la aplicación.
    - **`client/main.go`**: Código para iniciar el cliente de línea de comandos.
    - **`server/main.go`**: Código para iniciar el servidor.

3. **`internal/`**: Contiene la lógica de negocio y las interfaces.
    - **`domain/`**: Contiene la lógica de negocio y las interfaces.
        - **`go.mod`**: Módulo de Go para la capa de dominio.
        - **`ports/process.go`**: Define las interfaces (puertos) para la ejecución de procesos.
        - **`services/orchestrator.go`**: Implementación futura para la orquestación de procesos.

4. **`adapters/`**: Contiene las implementaciones de los adaptadores.
    - **`go.mod`**: Módulo de Go para la capa de adaptadores.
    - **`grpc/`**: Implementación gRPC.
        - **`protos/remote_process/remote_process.proto`**: Definiciones de Protocol Buffers.
        - **`client/remote_process/remote_process_client.go`**: Implementación del cliente gRPC.
        - **`server/remote_process/remote_process_server.go`**: Implementación del servidor gRPC.
    - **`local/process_executor.go`**: Implementación local para la ejecución de procesos.

Esta estructura modular permite que cada capa de la arquitectura hexagonal sea un módulo independiente, lo que facilita la gestión de dependencias y el desarrollo colaborativo. Además, el uso de `go.work` permite trabajar con todos los módulos en un solo espacio de trabajo, simplificando el desarrollo y las pruebas.

[Enlace al post original:](https://medium.com/@rubentxu/creando-un-mvp-de-ejecuci%C3%B3n-remota-de-procesos-con-grpc-y-arquitectura-hexagonal-f0daa1e33c17)


---

- Incluir configuracion de certificados para gRPC y TLS, creando un certificado autofirmado
- Incluir dependencias para trazas zap.Logger
- Incluir fichero config en cada módulo para recoger configuración centralizada