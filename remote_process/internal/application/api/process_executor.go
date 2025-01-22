package api

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

	"dev.rubentxu.devops-platform/remote_process/internal/ports"
)

// LocalProcessExecutor implementa ports.ProcessExecutor para la ejecución local de comandos.
type LocalProcessExecutor struct {
	processes      map[string]*exec.Cmd
	healthStatuses map[string]*ports.ProcessHealthStatus
	mu             sync.Mutex
	healthStatusMu sync.Mutex
}

// NewLocalProcessExecutor crea una nueva instancia de LocalProcessExecutor.
func NewLocalProcessExecutor() *LocalProcessExecutor {
	return &LocalProcessExecutor{
		processes:      make(map[string]*exec.Cmd),
		healthStatuses: make(map[string]*ports.ProcessHealthStatus),
	}
}

// Constantes para los tipos de mensaje
const (
	TypeSetup  = "SETUP"
	TypeInfo   = "INFO"
	TypeError  = "ERROR"
	TypeWarn   = "WARN"
	TypeDebug  = "DEBUG"
	TypeStdout = "STDOUT"
	TypeStderr = "STDERR"
	TypeHealth = "HEALTH"
)

// sendOutput es una función auxiliar para enviar mensajes formateados al canal de salida
func (e *LocalProcessExecutor) sendOutput(outputChan chan ports.ProcessOutput, processID string, messageType string, message string, isError bool) {
	formattedMessage := fmt.Sprintf("[RPC] %s", message)

	// Obtener el estado actual del proceso
	e.healthStatusMu.Lock()
	currentStatus := ports.UNKNOWN
	if status, exists := e.healthStatuses[processID]; exists {
		currentStatus = status.Status
	}
	e.healthStatusMu.Unlock()

	outputChan <- ports.ProcessOutput{
		ProcessID: processID,
		Output:    formattedMessage,
		IsError:   isError,
		Type:      messageType,
		Status:    currentStatus,
	}
}

// sendHealthStatus envía un mensaje de estado de salud a través del canal de output
func (e *LocalProcessExecutor) sendHealthStatus(outputChan chan ports.ProcessOutput, processID string, status ports.ProcessStatus, message string) {
	outputChan <- ports.ProcessOutput{
		ProcessID: processID,
		Output:    fmt.Sprintf("[RPC] %s", message),
		IsError:   false,
		Type:      TypeHealth,
		Status:    status,
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
		Setpgid:   true,
		Pdeathsig: syscall.SIGTERM,
	}

	outputChan := make(chan ports.ProcessOutput, 100)

	// Configurar variables de entorno
	userEnv := []string{}
	for k, v := range env {
		userEnv = append(userEnv, fmt.Sprintf("%s=%s", k, v))
	}

	// Enviar mensajes de configuración
	e.sendOutput(outputChan, processID, TypeSetup, fmt.Sprintf("Starting command: %v", command), false)
	e.sendOutput(outputChan, processID, TypeSetup, fmt.Sprintf("Environment variables: %v", userEnv), false)
	e.sendOutput(outputChan, processID, TypeSetup, fmt.Sprintf("Working directory: %v", dir), false)

	// Configurar el entorno
	cmd.Env = append(os.Environ(), userEnv...)

	// Configurar pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.sendOutput(outputChan, processID, TypeError, fmt.Sprintf("Failed to create stdout pipe: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		e.sendOutput(outputChan, processID, TypeError, fmt.Sprintf("Failed to create stderr pipe: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Iniciar proceso
	if err := cmd.Start(); err != nil {
		e.sendOutput(outputChan, processID, TypeError, fmt.Sprintf("Failed to start command: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	e.sendOutput(outputChan, processID, TypeInfo, fmt.Sprintf("Process started with PID: %d", cmd.Process.Pid), false)

	e.mu.Lock()
	e.processes[processID] = cmd
	e.mu.Unlock()

	// Inicializa el ProcessHealthStatus y envía el estado inicial
	e.healthStatusMu.Lock()
	e.healthStatuses[processID] = &ports.ProcessHealthStatus{
		ProcessID: processID,
		Status:    ports.RUNNING,
		Message:   fmt.Sprintf("Process started with PID: %d", cmd.Process.Pid),
	}
	e.healthStatusMu.Unlock()
	e.sendHealthStatus(outputChan, processID, ports.RUNNING, fmt.Sprintf("Process started with PID: %d", cmd.Process.Pid))

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

	// Goroutine para esperar a que el proceso termine
	stopMonitor := make(chan struct{})
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-stopMonitor:
				return
			case <-ticker.C:
				e.healthStatusMu.Lock()
				if status, exists := e.healthStatuses[processID]; exists {
					if cmd.ProcessState != nil {
						if cmd.ProcessState.Success() {
							status.Status = ports.FINISHED
							status.Message = "Process finished successfully"
						} else {
							status.Status = ports.ERROR
							status.Message = fmt.Sprintf("Process exited with error: %v", cmd.ProcessState.String())
						}
						e.sendHealthStatus(outputChan, processID, status.Status, status.Message)
					} else {
						// Verificar si el proceso sigue vivo
						if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
							status.Status = ports.ERROR
							status.Message = fmt.Sprintf("Process not responding: %v", err)
							e.sendHealthStatus(outputChan, processID, status.Status, status.Message)
						}
					}
				}
				e.healthStatusMu.Unlock()
			}
		}
	}()

	// Goroutine para esperar a que el proceso termine
	go func() {
		err := cmd.Wait()
		wg.Wait() // Esperar a que terminen de leer stdout/stderr

		// Detener la goroutine de monitoreo
		close(stopMonitor)

		e.healthStatusMu.Lock()
		var processResult string
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				e.healthStatuses[processID].Status = ports.ERROR
				processResult = fmt.Sprintf("Process exited with code %d: %v", exitErr.ExitCode(), err)
				e.sendHealthStatus(outputChan, processID, ports.ERROR, processResult)
			} else {
				e.healthStatuses[processID].Status = ports.ERROR
				processResult = fmt.Sprintf("Process failed: %v", err)
				e.sendHealthStatus(outputChan, processID, ports.ERROR, processResult)
			}
		} else {
			e.healthStatuses[processID].Status = ports.FINISHED
			processResult = "Process finished successfully"
			e.sendHealthStatus(outputChan, processID, ports.FINISHED, processResult)
		}
		e.healthStatusMu.Unlock()

		// Asegurar que todos los mensajes anteriores se han procesado
		time.Sleep(time.Second)

		// Enviar el último mensaje siempre con estado FINISHED
		outputChan <- ports.ProcessOutput{
			ProcessID: processID,
			Output:    fmt.Sprintf("[RPC] Process execution completed: %s with id %s", processResult, processID),
			IsError:   false,
			Type:      TypeHealth,
			Status:    ports.FINISHED,
		}

		// Esperar un momento para asegurar que el último mensaje se procesa
		time.Sleep(2 * time.Second)

		// Cerrar el canal después de enviar el último mensaje
		close(outputChan)
	}()

	return outputChan, nil
}

// readOutput lee de un pipe (stdout o stderr) y envía la salida al canal outputChan.
func (e *LocalProcessExecutor) readOutput(processID string, reader io.Reader, outputChan chan ports.ProcessOutput, isError bool) {
	messageType := TypeStdout
	if isError {
		messageType = TypeStderr
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		e.sendOutput(outputChan, processID, messageType, scanner.Text(), isError)
	}
	if err := scanner.Err(); err != nil {
		e.sendOutput(outputChan, processID, TypeError, fmt.Sprintf("Error reading output: %v", err), true)
	}
}

// Stop detiene un proceso en ejecución.
func (e *LocalProcessExecutor) Stop(ctx context.Context, processID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cmd, ok := e.processes[processID]
	if !ok {
		return fmt.Errorf("process not found: %s", processID)
	}

	// Actualizar el estado antes de intentar detener
	e.healthStatusMu.Lock()
	if status, exists := e.healthStatuses[processID]; exists {
		status.Status = ports.STOPPED
		status.Message = "Process stopping gracefully"
		if outputChan, ok := e.getOutputChannel(processID); ok {
			e.sendHealthStatus(outputChan, processID, ports.STOPPED, "Process stopping gracefully")
		}
	}
	e.healthStatusMu.Unlock()

	// Obtener el Process Group ID
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to get process group: %v", err)
	}

	// Primero intentar una terminación suave con SIGTERM
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		log.Printf("Warning: Failed to send SIGTERM to process group %d: %v", pgid, err)
	}

	// Esperar un poco para que el proceso termine suavemente
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Esperar hasta 5 segundos para que termine suavemente
	select {
	case err := <-done:
		if err != nil {
			log.Printf("[RPC][%s] Process %s exited with error: %v", TypeError, processID, err)
		} else {
			log.Printf("[RPC][%s] Process %s stopped gracefully", TypeInfo, processID)
		}
	case <-time.After(5 * time.Second):
		log.Printf("[RPC][%s] Process %s did not terminate after SIGTERM, sending SIGKILL", TypeWarn, processID)
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			log.Printf("[RPC][%s] Failed to send SIGKILL to process group %d: %v", TypeError, pgid, err)
		}
		<-done
	}

	// Limpiar recursos
	delete(e.processes, processID)
	e.healthStatusMu.Lock()
	if status, exists := e.healthStatuses[processID]; exists {
		status.ProcessID = processID
		status.Status = ports.STOPPED
		status.Message = "Process stopped"
	}
	e.healthStatusMu.Unlock()

	// Cerrar cualquier pipe o descriptor de archivo abierto
	if cmd.ProcessState == nil {
		if err := cmd.Process.Release(); err != nil {
			log.Printf("Warning: Failed to release process resources: %v", err)
		}
	}

	return nil
}

// MonitorHealth monitoriza el estado de un proceso.
func (e *LocalProcessExecutor) MonitorHealth(ctx context.Context, processID string, checkInterval int64) (<-chan ports.ProcessHealthStatus, error) {
	healthChan := make(chan ports.ProcessHealthStatus, 10)

	go func() {
		ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
		defer ticker.Stop()

		var lastStatus ports.ProcessStatus

		for {
			select {
			case <-ctx.Done():
				log.Printf("[RPC][%s] Health monitoring stopped for process %s: context cancelled", TypeInfo, processID)
				close(healthChan)
				return
			case <-ticker.C:
				e.healthStatusMu.Lock()
				healthStatus, ok := e.healthStatuses[processID]
				if !ok {
					e.healthStatusMu.Unlock()
					log.Printf("[RPC][%s] Health monitoring stopped for process %s: process not found", TypeInfo, processID)
					close(healthChan)
					return
				}

				// Verificar si el proceso sigue vivo
				cmd := e.processes[processID]
				if cmd != nil && cmd.Process != nil {
					if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
						healthStatus.Status = ports.ERROR
						healthStatus.Message = fmt.Sprintf("Process not responding: %v", err)
					}
				}

				// Copia el valor actual de ProcessHealthStatus
				currentStatus := *healthStatus
				e.healthStatusMu.Unlock()

				// Solo enviar si el estado ha cambiado o es FINISHED/ERROR
				if currentStatus.Status != lastStatus ||
					currentStatus.Status == ports.FINISHED ||
					currentStatus.Status == ports.ERROR {

					lastStatus = currentStatus.Status

					select {
					case healthChan <- currentStatus:
						log.Printf("[RPC][%s] Health status sent for process %s: %s - %s",
							TypeDebug, processID, currentStatus.Status, currentStatus.Message)

						if currentStatus.Status == ports.FINISHED || currentStatus.Status == ports.ERROR {
							time.Sleep(time.Second)
							log.Printf("[RPC][%s] Health monitoring completed for process %s with status: %s",
								TypeInfo, processID, currentStatus.Status)
							close(healthChan)
							return
						}
					default:
						log.Printf("[RPC][%s] Health channel blocked for process %s, skipping update",
							TypeWarn, processID)
					}
				}
			}
		}
	}()

	return healthChan, nil
}

// getOutputChannel es un helper para obtener el canal de output si existe
func (e *LocalProcessExecutor) getOutputChannel(processID string) (chan ports.ProcessOutput, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, exists := e.processes[processID]
	if !exists {
		return nil, false
	}
	// Aquí podrías mantener un mapa de canales de output si es necesario
	return nil, false
}
