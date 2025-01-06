package local

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"dev.rubentxu.devops-platform/domain/ports"
)

// LocalProcessExecutor implementa ports.ProcessExecutor para la ejecución local de comandos.
type LocalProcessExecutor struct {
	processes      map[string]*exec.Cmd
	healthStatuses map[string]*ports.HealthStatus
	mu             sync.Mutex
	healthStatusMu sync.Mutex
}

// NewLocalProcessExecutor crea una nueva instancia de LocalProcessExecutor.
func NewLocalProcessExecutor() *LocalProcessExecutor {
	return &LocalProcessExecutor{
		processes:      make(map[string]*exec.Cmd),
		healthStatuses: make(map[string]*ports.HealthStatus),
	}
}

// Start inicia un proceso localmente y devuelve un canal para la salida en streaming.
func (e *LocalProcessExecutor) Start(ctx context.Context, processID string, command []string, env map[string]string, dir string) (<-chan ports.ProcessOutput, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir

	// Configurar el proceso para ejecutarse en su propio grupo
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true, // Esto hace que el proceso tenga su propio grupo
	}

	// Canal para enviar la salida
	outputChan := make(chan ports.ProcessOutput)

	// Configura las variables de entorno solo con las proporcionadas por el usuario
	userEnv := []string{}
	for k, v := range env {
		userEnv = append(userEnv, fmt.Sprintf("%s=%s", k, v))
	}

	go func() {
		// Enviar la configuración del comando y las variables de entorno proporcionadas por el usuario por el canal de salida
		line := fmt.Sprintf("Starting command: %v with user-provided env: %v with directory: %v", command, userEnv, dir)
		outputChan <- ports.ProcessOutput{
			ProcessID: processID,
			Output:    line,
			IsError:   false,
		}
	}()

	// Configura las variables de entorno incluyendo las del sistema operativo
	cmd.Env = append(os.Environ(), userEnv...)

	// Captura la salida estándar y de error
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Inicia el proceso
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	e.mu.Lock()
	e.processes[processID] = cmd
	e.mu.Unlock()

	// Inicializa el HealthStatus
	e.healthStatusMu.Lock()
	e.healthStatuses[processID] = &ports.HealthStatus{
		ProcessID: processID,
		IsRunning: true,
		Status:    "Process started",
	}
	e.healthStatusMu.Unlock()

	// WaitGroup para esperar a que las goroutines terminen
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutines para leer la salida estándar y de error
	go func() {
		defer wg.Done()
		e.readOutput(processID, stdout, outputChan, false)
	}()
	go func() {
		defer wg.Done()
		e.readOutput(processID, stderr, outputChan, true)
	}()

	// Goroutine para esperar a que el proceso termine y cerrar el canal
	go func() {
		err := cmd.Wait()
		e.healthStatusMu.Lock()
		if err != nil {
			e.healthStatuses[processID].Status = fmt.Sprintf("Process finished with error: %v", err)
		} else {
			e.healthStatuses[processID].Status = "Process finished successfully"
		}
		e.healthStatuses[processID].IsRunning = false
		e.healthStatusMu.Unlock()

		// Espera a que las goroutines terminen antes de cerrar el canal
		wg.Wait()
		close(outputChan)
	}()

	return outputChan, nil
}

// readOutput lee de un pipe (stdout o stderr) y envía la salida al canal outputChan.
func (e *LocalProcessExecutor) readOutput(processID string, reader io.Reader, outputChan chan ports.ProcessOutput, isError bool) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		outputChan <- ports.ProcessOutput{
			ProcessID: processID,
			Output:    line,
			IsError:   isError,
		}
	}
	if err := scanner.Err(); err != nil {
		outputChan <- ports.ProcessOutput{
			ProcessID: processID,
			Output:    fmt.Sprintf("Error reading output: %v", err),
			IsError:   true,
		}
	}
}

// Stop detiene un proceso en ejecución.
func (e *LocalProcessExecutor) Stop(ctx context.Context, processID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cmd, ok := e.processes[processID]
	if !ok {
		return fmt.Errorf("process not found")
	}

	// Actualizar el estado de salud antes de detener
	e.healthStatusMu.Lock()
	if status, exists := e.healthStatuses[processID]; exists {
		status.IsRunning = false
		status.Status = "Process stopping"
	}
	e.healthStatusMu.Unlock()

	// Obtener el Process Group ID para matar todo el árbol de procesos
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to get process group: %v", err)
	}

	// Intentar primero una terminación suave
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		log.Printf("Warning: Failed to send SIGTERM to process group %d: %v", pgid, err)
	}

	// Esperar un poco para que el proceso termine suavemente
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		// Si no termina después de 5 segundos, forzar la terminación
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			log.Printf("Warning: Failed to send SIGKILL to process group %d: %v", pgid, err)
		}
	case err := <-done:
		if err != nil {
			log.Printf("Process %s exited with error: %v", processID, err)
		}
	}

	// Actualizar el estado final
	e.healthStatusMu.Lock()
	if status, exists := e.healthStatuses[processID]; exists {
		status.IsRunning = false
		status.Status = "Process stopped"
		status.IsHealthy = false
	}
	e.healthStatusMu.Unlock()

	// Eliminar el proceso del mapa
	delete(e.processes, processID)

	return nil
}

// MonitorHealth monitoriza el estado de un proceso.
func (e *LocalProcessExecutor) MonitorHealth(ctx context.Context, processID string, checkInterval int64) (<-chan ports.HealthStatus, error) {
	healthChan := make(chan ports.HealthStatus)

	go func() {
		ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				close(healthChan)
				return
			case <-ticker.C:
				e.healthStatusMu.Lock()
				healthStatus, ok := e.healthStatuses[processID]
				if !ok {
					e.healthStatusMu.Unlock()
					// Si el proceso ya no está en el mapa, cierra el canal y termina
					close(healthChan)
					return
				}
				// Copia el valor actual de HealthStatus
				currentStatus := *healthStatus
				e.healthStatusMu.Unlock()

				// Envía la copia al canal
				healthChan <- currentStatus
			}
		}
	}()

	return healthChan, nil
}
