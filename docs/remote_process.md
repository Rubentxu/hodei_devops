# Remote Process Service - Documentación Actualizada

## Índice

1. [Descripción General](#descripción-general)
2. [Guía para Principiantes](#guía-para-principiantes)
3. [API Técnica](#api-técnica)
4. [Ejemplos de Uso](#ejemplos-de-uso)
5. [Consideraciones de Seguridad](#consideraciones-de-seguridad)
6. [Opciones de Configuración](#opciones-de-configuración)
7. [Mejores Prácticas](#mejores-prácticas)
8. [Solución de Problemas](#solución-de-problemas)
9. [Recursos Adicionales](#recursos-adicionales)

---

## Descripción General

**Remote Process Service** es una solución integral que permite interactuar de forma remota con diferentes servidores o equipos. Sus principales características incluyen:

- **Ejecución de programas en computadoras remotas**  
  Inicia, visualiza y detén procesos como si estuvieras frente a la máquina.

- **Monitoreo de recursos del sistema**  
  Revisa indicadores de uso de CPU, memoria, disco y red en tiempo real.

- **Sincronización de archivos**  
  Transfiere y sincroniza archivos entre equipos de forma segura y eficiente.

Estas funcionalidades se basan en un enfoque altamente seguro, utilizando cifrado en las comunicaciones y autenticación robusta. El servicio está diseñado para adaptarse a distintos entornos, desde pequeñas implementaciones locales hasta infraestructuras distribuidas a gran escala.

---

## Guía para Principiantes

Esta sección busca orientar a quienes se inician en la herramienta, ofreciendo ejemplos simples para que puedan familiarizarse con sus funcionalidades principales.

### ¿Qué puedes hacer con este servicio?

1. **Ejecutar Programas Remotamente**
    - Funciona como un "control remoto" para tu servidor.
    - Permite ver la salida de los procesos en tiempo real.
    - Proporciona controles para pausar o detener programas cuando sea necesario.

2. **Ver el Estado de la Máquina**
    - Monitorear cuánta CPU está en uso.
    - Revisar la memoria disponible.
    - Consultar espacio en disco y actividad de red.
    - Obtener información de procesos y estadísticas de E/S (I/O).

3. **Manejar Archivos**
    - Copiar archivos entre computadoras de forma segura.
    - Mantener carpetas sincronizadas, similar a servicios en la nube pero con mayor flexibilidad.
    - Aplicar reglas de compresión, preservación de permisos y límites de ancho de banda.

### Ejemplos Sencillos

```go
// Iniciar un proceso remoto
client.StartProcess("mi-programa.sh", "/carpeta/trabajo")

// Obtener métricas básicas del sistema
metrics := client.CollectMetrics("cpu", "memory")

// Sincronizar archivos locales con un servidor remoto
client.SyncFiles("/ruta/local", "/ruta/remota")
```

---

## API Técnica

Para aquellos con un perfil técnico o desarrolladores que busquen integración avanzada, detallamos a continuación las principales llamadas gRPC, sus parámetros y niveles de acceso recomendados.

### 1. Gestión de Procesos

#### StartProcess
```protobuf
rpc StartProcess(stream ProcessStartRequest) returns (stream ProcessOutput)
```
- **Permisos**: admin, operator
- **Uso**: Inicia un proceso de forma remota y entrega la salida del mismo en tiempo real o bajo demanda.

#### StopProcess
```protobuf
rpc StopProcess(ProcessStopRequest) returns (ProcessStopResponse)
```
- **Permisos**: admin, operator
- **Uso**: Detiene un proceso que se esté ejecutando en el servidor remoto.

### 2. Monitorización

#### MonitorHealth
```protobuf
rpc MonitorHealth(stream HealthCheckRequest) returns (stream HealthStatus)
```
- **Permisos**: admin, operator, viewer
- **Métricas disponibles**:
    - CPU
    - Memoria
    - Disco
    - Red
    - Sistema
    - Procesos
    - I/O

Esta llamada gRPC permite suscribirse a un flujo de datos para observar en tiempo real la salud del sistema.

### 3. Sincronización de Archivos

#### SyncFiles
```protobuf
rpc SyncFiles(stream FileUploadRequest) returns (stream SyncProgress)
```
- **Características**:
    - Transferencia de archivos en chunks (fragmentos) para mayor confiabilidad.
    - Compresión opcional para optimizar el uso de ancho de banda.
    - Sincronización delta, ideal para actualizar únicamente los cambios.
    - Control de ancho de banda para no saturar la red.

---

## Ejemplos de Uso

Esta sección muestra ejemplos más detallados para diferentes escenarios de uso.

### 1. Monitoreo Básico

```go
// Recoge métricas de CPU y memoria de "servidor-1" cada 5 segundos
metrics, err := client.CollectMetrics(ctx, "servidor-1", []string{"cpu", "memory"}, 5)
if err != nil {
    log.Fatalf("Error al obtener métricas: %v", err)
}
```

### 2. Sincronización de Archivos

```go
// Sincroniza la carpeta "/local/docs" con "/remote/docs"
err := client.SyncDirectory(ctx, "/local/docs", "/remote/docs", &SyncOptions{
    Recursive: true,
    Compress:  true,
})
if err != nil {
    log.Fatalf("Error al sincronizar archivos: %v", err)
}
```

---

## Consideraciones de Seguridad

La seguridad es fundamental en Remote Process Service. Se recomienda revisar detenidamente las opciones de autenticación y cifrado para proteger la comunicación y los recursos.

### Autenticación y Autorización

- **JWT (JSON Web Tokens)**: Utilizados para autenticar a los usuarios, garantizando que solo personas con credenciales válidas accedan al sistema.
- **mTLS (Mutual TLS)**: Permite cifrar todo el tráfico y verificar la identidad tanto del cliente como del servidor.
- **Roles de usuario**:
    - `admin`: Acceso total a todas las operaciones.
    - `operator`: Capacidad para gestionar procesos y sincronizar archivos.
    - `viewer`: Acceso de solo lectura para monitoreo.

### Niveles de Acceso por Servicio

```go
accessibleRoles := map[string][]string{
    "/remote_process.RemoteProcessService/StartProcess": {"admin", "operator"},
    "/remote_process.RemoteProcessService/CollectMetrics": {"admin", "operator", "viewer"},
    "/remote_process.RemoteProcessService/SyncFiles": {"admin", "operator"},
}
```

---

## Opciones de Configuración

Ajustar las opciones de configuración adecuadamente puede optimizar el desempeño y la seguridad de la herramienta.

### Sincronización

- `recursive`: Incluye o no subdirectorios en la transferencia.
- `compress`: Activa la compresión de los archivos para ahorrar ancho de banda.
- `preserve_perms`: Mantiene los permisos de archivos y directorios originales.
- `bandwidth_limit`: Define un límite para el uso de red durante la sincronización.

### Monitoreo

- **Intervalos personalizables**: Ajusta la frecuencia con la que se recogen métricas.
- **Filtros de métricas**: Permite seleccionar solo aquellas métricas que te interesen.
- **Alertas configurables**: Posibilidad de definir umbrales de alerta (por ejemplo, CPU > 80%).

---

## Mejores Prácticas

1. **Seguridad**
    - Utiliza siempre mTLS para cifrar y autenticar las conexiones.
    - Rota los tokens JWT periódicamente para minimizar riesgos de filtraciones.
    - Otorga los permisos mínimos necesarios a cada rol.

2. **Rendimiento**
    - Utiliza compresión para transferir archivos grandes y reducir el consumo de ancho de banda.
    - Ajusta los intervalos de monitoreo para evitar saturar la red o los sistemas con demasiadas peticiones.
    - Configura límites de recursos (CPU, RAM) para prevenir bloqueos.

3. **Mantenimiento**
    - Supervisa los logs del servicio para identificar problemas tempranamente.
    - Realiza copias de seguridad (backups) de manera regular.
    - Mantén certificados y claves al día para garantizar la validez de las conexiones seguras.

---

## Solución de Problemas

A continuación, se describen algunos escenarios comunes y cómo resolverlos.

1. **Conexión Rechazada**
    - Verifica que los certificados estén vigentes y sean reconocidos.
    - Revisa que el token JWT no haya expirado.
    - Asegúrate de que el firewall permita el tráfico en los puertos correspondientes.

2. **Rendimiento Lento**
    - Reduce el tamaño de los chunks de transferencia o ajusta los parámetros de compresión.
    - Verifica el ancho de banda disponible.
    - Revisa que el servidor no esté saturado en CPU o memoria.

3. **Errores de Sincronización**
    - Confirma los permisos de lectura y escritura en ambas rutas (local y remota).
    - Asegúrate de tener suficiente espacio en disco.
    - Consulta los logs para identificar errores específicos o eventos relevantes.

---

## Recursos Adicionales

- [Código Fuente](https://github.com/tu-repo)
- [Ejemplos](https://github.com/tu-repo/examples)
- [Guía de Contribución](CONTRIBUTING.md)

Esta documentación está diseñada para ofrecer una visión completa de Remote Process Service, 
abarcando desde los primeros pasos para usuarios nuevos hasta la información técnica detallada para desarrolladores y 
administradores de sistemas. Te invitamos a explorar los repositorios y recursos adicionales para 
sacar el máximo provecho de esta herramienta. ¡Éxito en tus proyectos!