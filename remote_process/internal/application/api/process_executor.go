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
	prefixSetup  = "SETUP"
	prefixInfo   = "INFO"
	prefixError  = "ERROR"
	prefixWarn   = "WARN"
	prefixDebug  = "DEBUG"
	prefixStdout = "STDOUT"
	prefixStderr = "STDERR"
)

// sendOutput es una función auxiliar para enviar mensajes formateados al canal de salida
func (e *LocalProcessExecutor) sendOutput(outputChan chan ports.ProcessOutput, processID string, messageType string, message string, isError bool) {
	formattedMessage := fmt.Sprintf("[RPC][%s] %s", messageType, message)
	outputChan <- ports.ProcessOutput{
		ProcessID: processID,
		Output:    formattedMessage,
		IsError:   isError,
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
	e.sendOutput(outputChan, processID, prefixSetup, fmt.Sprintf("Starting command: %v", command), false)
	e.sendOutput(outputChan, processID, prefixSetup, fmt.Sprintf("Environment variables: %v", userEnv), false)
	e.sendOutput(outputChan, processID, prefixSetup, fmt.Sprintf("Working directory: %v", dir), false)

	// Configurar el entorno
	cmd.Env = append(os.Environ(), userEnv...)

	// Configurar pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		e.sendOutput(outputChan, processID, prefixError, fmt.Sprintf("Failed to create stdout pipe: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		e.sendOutput(outputChan, processID, prefixError, fmt.Sprintf("Failed to create stderr pipe: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Iniciar proceso
	if err := cmd.Start(); err != nil {
		e.sendOutput(outputChan, processID, prefixError, fmt.Sprintf("Failed to start command: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to start command: %v", err)
	}

	e.sendOutput(outputChan, processID, prefixInfo, fmt.Sprintf("Process started with PID: %d", cmd.Process.Pid), false)

	e.mu.Lock()
	e.processes[processID] = cmd
	e.mu.Unlock()

	// Inicializa el ProcessHealthStatus
	e.healthStatusMu.Lock()
	e.healthStatuses[processID] = &ports.ProcessHealthStatus{
		ProcessID: processID,
		Status:    ports.RUNNING,
		Message:   fmt.Sprintf("Process started with PID: %d", cmd.Process.Pid),
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

	// Goroutine para esperar a que el proceso termine
	go func() {
		err := cmd.Wait()
		wg.Wait()
		log.Printf("[RPC][%s] Process wait completed for process %s", prefixInfo, processID)

		e.healthStatusMu.Lock()
		defer e.healthStatusMu.Unlock()

		finalStatus := ports.FINISHED
		var finalMessage string

		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				finalStatus = ports.ERROR
				finalMessage = fmt.Sprintf("Process exited with code %d: %v", exitErr.ExitCode(), err)
				e.sendOutput(outputChan, processID, prefixError, finalMessage, true)
			} else {
				finalStatus = ports.ERROR
				finalMessage = fmt.Sprintf("Process failed: %v", err)
				e.sendOutput(outputChan, processID, prefixError, finalMessage, true)
			}
		} else {
			finalMessage = "Process finished successfully"
			e.sendOutput(outputChan, processID, prefixInfo, finalMessage, false)
		}

		// Actualizar el estado final
		e.healthStatuses[processID] = &ports.ProcessHealthStatus{
			ProcessID: processID,
			Status:    finalStatus,
			Message:   finalMessage,
		}

		// Asegurarse de que el estado final se propague
		select {
		case healthChan <- e.healthStatuses[processID]:
			e.logMessage(prefixInfo, processID, "Final status sent: %s", finalStatus)
		case <-time.After(time.Second):
			e.logMessage(prefixWarn, processID, "Could not send final status: %s", finalStatus)
		}

		// Esperar un momento para asegurar que el estado se ha propagado
		time.Sleep(time.Second)

		e.sendOutput(outputChan, processID, prefixInfo, "Output channel closing", false)
		time.Sleep(100 * time.Millisecond)
		log.Printf("[RPC][%s] Output channel closing for process %s", prefixInfo, processID)
		close(outputChan)
	}()

	return outputChan, nil
}

// readOutput lee de un pipe (stdout o stderr) y envía la salida al canal outputChan.
func (e *LocalProcessExecutor) readOutput(processID string, reader io.Reader, outputChan chan ports.ProcessOutput, isError bool) {
	prefix := prefixStdout
	if isError {
		prefix = prefixStderr
	}

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		e.sendOutput(outputChan, processID, prefix, scanner.Text(), isError)
	}
	if err := scanner.Err(); err != nil {
		e.sendOutput(outputChan, processID, prefixError, fmt.Sprintf("Error reading output: %v", err), true)
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
		status.Status = ports.STOPPING
		status.Message = "Process stopping gracefully"
	}
	e.healthStatusMu.Unlock()

	// Obtener el Process Group ID
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		return fmt.Errorf("failed to get process group: %v", err)
	}

	// Primero intentar una terminación suave con SIGTERM
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		e.logMessage(prefixWarn, processID, "Failed to send SIGTERM to process group %d: %v", pgid, err)
	}

	// Esperar un poco para que el proceso termine suavemente
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var stopErr error
	// Esperar hasta 5 segundos para que termine suavemente
	select {
	case err := <-done:
		if err != nil {
			e.logMessage(prefixError, processID, "Process exited with error: %v", err)
			stopErr = err
		} else {
			e.logMessage(prefixInfo, processID, "Process stopped gracefully")
		}
	case <-time.After(5 * time.Second):
		e.logMessage(prefixWarn, processID, "Process did not terminate after SIGTERM, sending SIGKILL")
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			e.logMessage(prefixError, processID, "Failed to send SIGKILL to process group %d: %v", pgid, err)
			stopErr = err
		}
		<-done
	}

	// Actualizar el estado final después de la detención
	e.healthStatusMu.Lock()
	if status, exists := e.healthStatuses[processID]; exists {
		if stopErr != nil {
			status.Status = ports.ERROR
			status.Message = fmt.Sprintf("Process stopped with error: %v", stopErr)
		} else {
			status.Status = ports.FINISHED
			status.Message = "Process stopped successfully"
		}
	}
	e.healthStatusMu.Unlock()

	// Limpiar recursos
	delete(e.processes, processID)

	return stopErr
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
				log.Printf("[RPC][%s] Health monitoring stopped for process %s: context cancelled", prefixInfo, processID)
				close(healthChan)
				return
			case <-ticker.C:
				e.healthStatusMu.Lock()
				healthStatus, ok := e.healthStatuses[processID]
				if !ok {
					e.healthStatusMu.Unlock()
					log.Printf("[RPC][%s] Health monitoring stopped for process %s: process not found", prefixInfo, processID)
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
							prefixDebug, processID, currentStatus.Status, currentStatus.Message)

						if currentStatus.Status == ports.FINISHED || currentStatus.Status == ports.ERROR {
							time.Sleep(time.Second)
							log.Printf("[RPC][%s] Health monitoring completed for process %s with status: %s",
								prefixInfo, processID, currentStatus.Status)
							close(healthChan)
							return
						}
					default:
						log.Printf("[RPC][%s] Health channel blocked for process %s, skipping update",
							prefixWarn, processID)
					}
				}
			}
		}
	}()

	return healthChan, nil
}
