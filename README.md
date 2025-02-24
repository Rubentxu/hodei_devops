# Hodei DevOps
<div align="center" >
  <img src="docs/assets/imagen-readme.png" alt="imagen-3" style="width: 100%;"/>
</div>

Hodei DevOps es un proyecto **open source** que forma parte de una idea orientada a transformar la forma en que se ejecutan 
y gestionan procesos en entornos devops y cloud native. 
Inspirado en las mejores prácticas de plataformas modernas, Hodei DevOps se centra en proporcionar una solución robusta 
y flexible para la orquestacion de ejecución de tareas remotas, y otros servicios para facilitar la gestión de flujos de trabajo con DevOps.

### Significado del nombre de Hodei
Uno de los significados que se asocian al nombre de Hodei en Euskadi es nube.
Esa significación se toma como referencia a los cielos cambiantes que se ven en la naturaleza. 
Otros aspectos que se relacionan con este nombre son la fugacidad de la belleza y la tendencia al cambio. 
Etimológicamente, también tiene el significado de Dios.

Tampoco pasa desapercibido el hecho de que a las personas que comparten el nombre de Hodei se les asocian aspectos como el ser soñadoras, tranquilas y sensibles. 
Todos estos rasgos han contribuido a que el nombre de Hodei perviva con fuerza a pesar del paso del tiempo.

## Arquitectura y Tecnologías

Hodei DevOps se basa en dos pilares tecnológicos clave:

- **gRPC:** Utilizamos este framework de alto rendimiento para implementar RPC (Remote Procedure Call) 
- con capacidades de streaming. Esto garantiza una comunicación eficiente y con baja latencia entre los componentes del sistema.
- **Arquitectura Hexagonal (Puertos y Adaptadores):** Esta arquitectura desacopla la lógica de negocio de los detalles
- de implementación (como la comunicación gRPC o la ejecución local de procesos), lo que facilita el mantenimiento, la evolución y las pruebas del sistema.
- **Go:** Lenguaje de programación utilizado para implementar los servicios de Hodei DevOps. Go es conocido por su eficiencia, simplicidad y facilidad de uso en entornos de sistemas y servicios distribuidos.

### Estructura del Proyecto


El proyecto está organizado utilizando Go y se gestiona mediante `go.work` para trabajar de manera modular con múltiples componentes o subproyectos.

## Requisitos Previos

Antes de comenzar, asegúrate de tener instalado lo siguiente:

- **Go:** Conocimientos básicos de Go y su entorno de desarrollo.
- **Protocol Buffers Compiler (protoc):** Necesario para compilar las definiciones de gRPC.
- **Línea de Comandos:** Familiaridad con el uso de terminal o consola.
- **Editor de Código:** Se recomienda VS Code con la extensión de Go, aunque cualquier editor es válido.

## Instalación

1. **Clona el repositorio:**
```bash
git clone https://github.com/tu-usuario/hodei-devops.git
cd hodei-devops
```

2. Instala las dependencias:
```bash
go mod tidy
```

## Comandos Clave en Makefile para el Desarrollo

Para facilitar el flujo de trabajo y agilizar las tareas comunes durante el desarrollo, se han configurado varios comandos en el Makefile. 
A continuación se explica cada uno de los más relevantes:

- **make proto**  
  Genera el código en Go a partir de los archivos Protocol Buffers (.proto). Este comando crea los archivos necesarios (`.pb.go` y `.grpc.pb.go`) para que los servicios gRPC funcionen correctamente.

- **make build**  
  Compila los binarios de los diferentes servicios (remote_process, orchestrator y archiva-go). El resultado se ubica en el directorio `bin`, facilitando su ejecución y despliegue.

- **make test**  
  Ejecuta una serie de pasos:
   1. Genera el código proto y compila los binarios.
   2. Inicia los servicios `remote_process` y `orchestrator`.
   3. Ejecuta el script de pruebas (`testProcess.sh`), que valida la funcionalidad general del sistema.

- **make test-go**  
  Ejecuta las pruebas unitarias e integradas escritas en Go:
   1. Detiene cualquier instancia previa de los servicios.
   2. Reconstruye y vuelve a iniciar los servicios.
   3. Ejecuta `go test` sobre los tests definidos en el directorio `tests`.
   4. Detiene automáticamente los servicios al finalizar las pruebas.

- **make test-integration**  
  Ejecuta las pruebas de integración ubicadas en el subdirectorio `tests/integration/`. Esto permite validar el comportamiento del sistema en escenarios más completos y realistas.

- **make clean**  
  Detiene los servicios en ejecución y elimina los binarios generados en el directorio `bin`, facilitando una limpieza del entorno de desarrollo.

- **make certs-dev**  
  Genera los certificados y claves necesarios para el entorno de desarrollo. Si los certificados ya existen, notifica que están disponibles. Esto es esencial para habilitar la comunicación segura (TLS) entre los servicios.

- **make run-remote_process**  
  Inicia el servicio `remote_process` en modo desarrollo, configurando variables de entorno necesarias (como rutas de certificados, puerto y JWT). Además, redirige la salida a un archivo de log y guarda el PID para facilitar su gestión.

- **make run-orchestrator**  
  Arranca el servicio `orchestrator` con las configuraciones de TLS y JWT en modo desarrollo, redirigiendo la salida a un log y almacenando el PID correspondiente.

- **make run-archiva-go**  
  Ejecuta el binario del servicio `archiva-go`, de forma similar a los otros servicios, permitiendo su monitoreo mediante logs y gestión a través del archivo de PID.

- **make stop-remote_process, stop-orchestrator, stop-archiva-go**  
  Cada uno de estos comandos se encarga de detener el servicio correspondiente usando el PID almacenado en el directorio `bin`. Esto permite reiniciar los servicios de forma controlada durante el desarrollo o en caso de incidencias.


- **make test-docker**  
  Ejecuta las pruebas en contenedores Docker, verificando que el entorno de Docker esté correctamente configurado y permitiendo la ejecución de pruebas de forma aislada y reproducible.

Estos comandos están diseñados para automatizar y simplificar tareas repetitivas, 
garantizando que el entorno de desarrollo se mantenga consistente y seguro. 
Utilízalos para construir, probar y desplegar el proyecto de forma rápida y eficiente.



### Contribuciones
Hodei DevOps es un proyecto colaborativo y abierto. ¡Tus contribuciones son bienvenidas! Para participar:

Haz un fork del repositorio.
Crea una rama para tu nueva funcionalidad o corrección.
Realiza tus cambios y haz commit con mensajes claros.
Envía un pull request para que la comunidad pueda revisar y fusionar tus cambios.
Consulta el archivo CONTRIBUTING.md para obtener más detalles sobre el proceso de contribución.


### Licencia
Este proyecto se distribuye bajo la licencia MIT. Revisa el archivo de licencia para más información.

### Contacto
Si tienes preguntas, sugerencias o deseas colaborar, ponte en contacto con nosotros:

Correo: rubentxu74@gmail.com
GitHub: Hodei DevOps

---

### Para instalar el chart:

```bash
# Desarrollo
helm install hodei-dev ./hodei-devops-chart \
  --set global.environment=development \
  --set jwt.token=<tu-token-jwt> \
  --set jwt.secret=<tu-secreto-jwt>

# Producción
helm install hodei-prod ./hodei-devops-chart \
  --set global.environment=production \
  --set jwt.token=<tu-token-jwt> \
  --set jwt.secret=<tu-secreto-jwt> \
  --set ingress.hosts[0].host=hodei.yourdomain.com
```

Todos los recursos desplegados tendrán el prefijo "hodei-devops" y 
las etiquetas apropiadas para identificarlos como parte de la aplicación Hodei DevOps.

helm install hodei-prod ./hodei-devops-chart -f custom-values.yaml