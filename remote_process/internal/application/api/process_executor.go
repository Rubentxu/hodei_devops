package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
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

// getProcessStatus obtiene el estado actual del proceso de forma segura
func (e *LocalProcessExecutor) getProcessStatus(processID string) ports.ProcessStatus {
	e.healthStatusMu.Lock()
	defer e.healthStatusMu.Unlock()
	if status, exists := e.healthStatuses[processID]; exists {
		return status.Status
	}
	return ports.UNKNOWN
}

// readOutput lee de un pipe (stdout o stderr) y envía la salida al canal outputChan.
func (e *LocalProcessExecutor) readOutput(ctx context.Context, processID string, reader io.Reader, outputChan chan ports.ProcessOutput, isError bool) {
	scanner := bufio.NewScanner(reader)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if !scanner.Scan() {
				return
			}
			messageType := TypeStdout
			if isError {
				messageType = TypeStderr
			}
			select {
			case <-ctx.Done():
				return
			case outputChan <- ports.ProcessOutput{
				ProcessID: processID,
				Output:    scanner.Text(),
				IsError:   isError,
				Type:      messageType,
				Status:    e.getProcessStatus(processID),
			}:
			}
		}
	}
}

// Start inicia un proceso localmente y devuelve un canal para la salida en streaming.
func (e *LocalProcessExecutor) Start(ctx context.Context, processID string, command []string, env map[string]string, dir string) (<-chan ports.ProcessOutput, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("no command provided")
	}
	outputChan := make(chan ports.ProcessOutput, 100)
	ctx, cancel := context.WithCancel(ctx)

	// Detectar si el comando ya usa un shell
	usesShell := false
	shellCommands := []string{"bash", "sh", "zsh", "ksh", "csh"}
	for _, sc := range shellCommands {
		if command[0] == sc {
			usesShell = true
			break
		}
	}

	var cmd *exec.Cmd

	if usesShell {
		// Si ya usa shell, ejecutar el comando tal cual
		cmd = exec.CommandContext(ctx, command[0], command[1:]...)
	} else {
		// Si no usa shell, envolver en bash -c para expansión de variables
		fullCommand := strings.Join(command, " ")
		cmd = exec.CommandContext(ctx, "bash", "-c", fullCommand)
		e.sendOutput(outputChan, processID, TypeDebug, "Comando envuelto en bash -c", false)
	}

	cmd.Dir = dir

	// Configurar el proceso para ejecutarse en su propio grupo
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGTERM,
	}

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
		cancel()
		e.sendOutput(outputChan, processID, TypeError, fmt.Sprintf("Failed to create stdout pipe: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		e.sendOutput(outputChan, processID, TypeError, fmt.Sprintf("Failed to create stderr pipe: %v", err), true)
		close(outputChan)
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	// Iniciar proceso
	if err := cmd.Start(); err != nil {
		cancel()
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
	wg.Add(3) // stdout, stderr, y health monitor

	// Goroutines para leer la salida estándar y de error
	go func() {
		defer wg.Done()
		e.readOutput(ctx, processID, stdout, outputChan, false)
	}()

	go func() {
		defer wg.Done()
		e.readOutput(ctx, processID, stderr, outputChan, true)
	}()

	// Goroutine para monitoreo de salud
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
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

		// Cancelar el contexto para detener todas las goroutines
		cancel()

		// Actualizar el estado final del proceso
		e.healthStatusMu.Lock()
		if status, exists := e.healthStatuses[processID]; exists {
			if err != nil {
				status.Status = ports.ERROR
				status.Message = fmt.Sprintf("Process exited with error: %v", err)
			} else {
				status.Status = ports.FINISHED
				status.Message = "Process finished successfully"
			}
			e.sendHealthStatus(outputChan, processID, status.Status, status.Message)
		}
		e.healthStatusMu.Unlock()

		// Esperar a que todas las goroutines terminen
		wg.Wait()

		// Limpiar recursos
		e.mu.Lock()
		delete(e.processes, processID)
		e.mu.Unlock()

		e.healthStatusMu.Lock()
		delete(e.healthStatuses, processID)
		e.healthStatusMu.Unlock()

		// Cerrar el canal de salida
		close(outputChan)
	}()

	return outputChan, nil
}

// Stop detiene un proceso en ejecución.
func (e *LocalProcessExecutor) Stop(ctx context.Context, processID string) error {
	e.mu.Lock()
	cmd, exists := e.processes[processID]
	e.mu.Unlock()

	if !exists {
		return fmt.Errorf("process %s not found", processID)
	}

	// Actualizar estado a STOPPED
	e.healthStatusMu.Lock()
	if status, exists := e.healthStatuses[processID]; exists {
		status.Status = ports.STOPPED
		status.Message = "Process stopped by user"
	}
	e.healthStatusMu.Unlock()

	// Intentar terminar el proceso de forma elegante
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Printf("Failed to send SIGTERM to process %s: %v", processID, err)
		// Si falla SIGTERM, forzar la terminación con SIGKILL
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %s: %v", processID, err)
		}
	}

	// Esperar un tiempo razonable para que el proceso termine
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(5 * time.Second):
		if err := cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process %s after timeout: %v", processID, err)
		}
		return fmt.Errorf("process %s killed after timeout", processID)
	case err := <-done:
		if err != nil {
			return fmt.Errorf("process %s stopped with error: %v", processID, err)
		}
		return nil
	}
}

// MonitorHealth monitoriza el estado de un proceso.
func (e *LocalProcessExecutor) MonitorHealth(ctx context.Context, processID string, checkInterval int64) (<-chan ports.ProcessHealthStatus, error) {
	healthChan := make(chan ports.ProcessHealthStatus, 10)

	e.mu.Lock()
	_, exists := e.processes[processID]
	e.mu.Unlock()

	if !exists {
		close(healthChan)
		return nil, fmt.Errorf("process %s not found", processID)
	}

	go func() {
		defer close(healthChan)
		ticker := time.NewTicker(time.Duration(checkInterval) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.healthStatusMu.Lock()
				if status, exists := e.healthStatuses[processID]; exists {
					select {
					case <-ctx.Done():
						e.healthStatusMu.Unlock()
						return
					case healthChan <- *status:
					}
				}
				e.healthStatusMu.Unlock()
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
