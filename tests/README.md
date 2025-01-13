# Tests del Sistema de Ejecución de Procesos Remotos

Este directorio contiene las pruebas automatizadas para el sistema de ejecución de procesos remotos. 
Los tests están diseñados para validar diferentes aspectos del sistema y verificar su comportamiento bajo diversas condiciones.

## Estructura de Tests

### Tests Básicos de Conectividad y Comunicación
- **Basic Echo**: Verifica la comunicación básica y el pipeline de ejecución.
- **Slow Process**: Prueba el manejo de procesos con latencia y delays de red.

### Tests de Carga y Recursos
- **CPU Intensive**: Verifica el comportamiento con procesos que consumen mucha CPU.
- **CPU Load Monitor**: Monitorea la carga del sistema durante la ejecución.
- **System Resources**: Recopila información sobre recursos del sistema.
- **Memory Intensive**: Prueba el manejo de procesos con alto consumo de memoria.

### Tests de E/S y Bloqueo
- **IO Blocking**: Verifica el comportamiento con operaciones de E/S intensivas.
- **File Descriptors**: Prueba los límites de descriptores de archivo.

### Tests de Manejo de Errores
- **Invalid Command**: Verifica el manejo de comandos inexistentes.
- **Permission Denied**: Prueba el manejo de errores de permisos.
- **Process Recovery**: Verifica la recuperación tras fallos.

### Tests de Contexto y Ambiente
- **Environment Variables**: Verifica la propagación de variables de entorno.
- **Working Directory**: Prueba la ejecución en directorios específicos.

### Tests de Red y Conectividad
- **Network Connectivity**: Verifica operaciones de red.
- **DNS Resolution**: Prueba resolución de nombres de dominio.

### Tests de Procesos y Concurrencia
- **Process Tree**: Verifica el manejo de procesos anidados.
- **Concurrent Operations**: Prueba operaciones concurrentes.
- **Long Running Processes**: Verifica el manejo de procesos largos y su terminación.

## Aspectos Probados

1. **Confiabilidad de Red**
    - Manejo de latencia
    - Timeouts
    - Reconexiones
    - Pérdida de conexión

2. **Gestión de Recursos**
    - Uso de CPU
    - Consumo de memoria
    - Descriptores de archivo
    - Límites del sistema

3. **Manejo de Errores**
    - Errores de comando
    - Errores de permisos
    - Timeouts
    - Recuperación de fallos

4. **Monitoreo y Estado**
    - Estado de procesos
    - Métricas del sistema
    - Logs y salida de procesos

5. **Concurrencia y Escalabilidad**
    - Múltiples procesos
    - Operaciones paralelas
    - Gestión de recursos compartidos

## Ejecución de Tests

## Monitoreo Global

El sistema incluye un monitor global que permite observar el estado de todos los procesos en tiempo real, proporcionando:
- Estado de cada proceso
- Salida en tiempo real
- Métricas de rendimiento
- Logs de eventos

## Consideraciones de Diseño

Los tests están diseñados teniendo en cuenta las falacias de la computación distribuida:
1. La red es confiable
2. La latencia es cero
3. El ancho de banda es infinito
4. La red es segura
5. La topología no cambia
6. Hay un administrador
7. El transporte es homogéneo
8. La red es homogénea

## Contribución

Al agregar nuevos tests, considerar:
1. Cobertura de casos límite
2. Escenarios de error
3. Condiciones de carrera
4. Timeouts y cancelaciones
5. Limpieza de recursos