package grpc

import (
	"bytes"
	"context"
	"dev.rubentxu.devops-platform/protos/remote_process"
	"dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc/security"
	"dev.rubentxu.devops-platform/remote_process/internal/ports"
	"encoding/json"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"os"
	"os/exec"

	"crypto/tls"
	"log"
	"net"
)

// ServerAdapter implementa la interfaz RemoteProcessServiceServer generada por protoc
type ServerAdapter struct {
	remote_process.UnimplementedRemoteProcessServiceServer
	executor       ports.ProcessExecutor
	metricsHandler *MetricsHandler
	syncHandler    *SyncHandler
	port           string
	server         *grpc.Server
}

func NewAdapter(
	executor ports.ProcessExecutor,
	metricsService ports.MetricsService,
	syncService ports.SyncService,
	port string,
) *ServerAdapter {
	return &ServerAdapter{
		executor:       executor,
		metricsHandler: NewMetricsHandler(metricsService),
		syncHandler:    NewSyncHandler(syncService),
		port:           port,
	}
}

// StartProcess implementa la lógica para iniciar un proceso con streaming de salida
func (s *ServerAdapter) StartProcess(stream remote_process.RemoteProcessService_StartProcessServer) error {
	// Recibir la solicitud inicial
	in, err := stream.Recv()
	if err != nil {
		return err
	}

	// Extraer el processID de la solicitud inicial
	processID := in.ProcessId

	// Iniciar el proceso
	outputChan, err := s.executor.Start(stream.Context(), processID, in.Command, in.Environment, in.WorkingDirectory)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to start process: %v", err)
	}

	// Transmitir la salida del proceso
	for output := range outputChan {
		err := stream.Send(&remote_process.ProcessOutput{
			ProcessId: output.ProcessID,
			Output:    output.Output,
			IsError:   output.IsError,
			Type:      output.Type,
			Status:    ConvertPortsProcessStatusToProto(output.Status),
		})
		if err != nil {
			return status.Errorf(codes.Internal, "failed to send output: %v", err)
		}
	}

	return nil
}

// StopProcess implementa la lógica para detener un proceso
func (s *ServerAdapter) StopProcess(ctx context.Context, req *remote_process.ProcessStopRequest) (*remote_process.ProcessStopResponse, error) {
	err := s.executor.Stop(ctx, req.ProcessId)
	if err != nil {
		log.Printf("Error stopping process: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to stop process: %v", err)
	}

	return &remote_process.ProcessStopResponse{Success: true, Message: "Process stopped successfully"}, nil
}

// MonitorHealth implementa la lógica para monitorizar el estado de un proceso
func (s *ServerAdapter) MonitorHealth(stream remote_process.RemoteProcessService_MonitorHealthServer) error {
	// Recibe el primer HealthCheckRequest
	in, err := stream.Recv()
	if err != nil {
		return err
	}
	processID := in.ProcessId
	checkInterval := in.CheckInterval

	// Utiliza un canal para recibir ProcessHealthStatus del método MonitorHealth
	healthChan, err := s.executor.MonitorHealth(stream.Context(), processID, checkInterval)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to start health monitoring: %v", err)
	}

	// Transmite actualizaciones de ProcessHealthStatus al cliente
	for healthStatus := range healthChan {
		err := stream.Send(&remote_process.ProcessHealthStatus{
			ProcessId: healthStatus.ProcessID,
			Status:    ConvertPortsProcessStatusToProto(healthStatus.Status),
			Message:   healthStatus.Message,
		})
		if err != nil {
			return status.Errorf(codes.Internal, "failed to send health status: %v", err)
		}
		log.Printf("Health status: %v", healthStatus) // Log the health status
	}

	return nil
}

func (s *ServerAdapter) CollectMetrics(stream remote_process.RemoteProcessService_CollectMetricsServer) error {
	return s.metricsHandler.CollectMetrics(stream)
}

func (s *ServerAdapter) SyncFiles(req *remote_process.FileSyncRequest, stream remote_process.RemoteProcessService_SyncFilesServer) error {
	return s.syncHandler.SyncFiles(req, stream)
}

func (s *ServerAdapter) SyncFilesStream(stream remote_process.RemoteProcessService_SyncFilesStreamServer) error {
	return s.syncHandler.SyncFilesStream(stream)
}

func (s *ServerAdapter) GetRemoteFileList(ctx context.Context, req *remote_process.RemoteListRequest) (*remote_process.RemoteFileList, error) {
	return s.syncHandler.GetRemoteFileList(ctx, req)
}

func (s *ServerAdapter) DeleteRemoteFile(ctx context.Context, req *remote_process.RemoteDeleteRequest) (*remote_process.RemoteDeleteResponse, error) {
	return s.syncHandler.DeleteRemoteFile(ctx, req)
}

func (s *ServerAdapter) Start(tlsConfig *tls.Config, authInterceptor *security.AuthInterceptor, env string) {
	listen, err := net.Listen("tcp", s.port)
	if err != nil {
		log.Fatalf("failed to listen on port %s, error: %v", s.port, err)
	}

	// Crear opciones del servidor con TLS y autenticación
	opts := []grpc.ServerOption{
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.UnaryInterceptor(authInterceptor.Unary()),
		grpc.StreamInterceptor(authInterceptor.Stream()),
	}

	log.Printf("starting with options: %v", opts)
	grpcServer := grpc.NewServer(opts...)
	s.server = grpcServer

	// Registrar servicios de negocio gRPC
	remote_process.RegisterRemoteProcessServiceServer(grpcServer, s)

	// Registrar el servicio de reflexión en entorno de desarrollo
	if env == "development" {
		reflection.Register(grpcServer)
		log.Println("gRPC reflection service registered in development environment")
	}

	// Registrar el servicio de salud gRPC
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Establecer el estado del servidor como "SERVING"
	healthServer.SetServingStatus("grpc.health.v1.server", grpc_health_v1.HealthCheckResponse_SERVING)

	log.Printf("starting remote process service on port %s with mTLS and JWT auth...", s.port)
	if err := grpcServer.Serve(listen); err != nil {
		log.Fatalf("failed to serve grpc on port %s: %v", s.port, err)
	}
}

func (s *ServerAdapter) Stop() {
	if s.server != nil {
		s.server.GracefulStop()
	}
}

// ConvertPortsProcessStatusToProto converts ports.ProcessStatus to remote_process.ProcessStatus
func ConvertPortsProcessStatusToProto(status ports.ProcessStatus) remote_process.ProcessStatus {
	switch status {
	case ports.UNKNOWN:
		return remote_process.ProcessStatus_UNKNOWN_PROCESS_STATUS
	case ports.RUNNING:
		return remote_process.ProcessStatus_RUNNING
	case ports.HEALTHY:
		return remote_process.ProcessStatus_HEALTHY
	case ports.ERROR:
		return remote_process.ProcessStatus_ERROR
	case ports.STOPPED:
		return remote_process.ProcessStatus_STOPPED
	case ports.FINISHED:
		return remote_process.ProcessStatus_FINISHED
	default:
		return remote_process.ProcessStatus_UNKNOWN_PROCESS_STATUS
	}
}

// ExecuteCommand: Ejecuta un comando remoto, lee stdout/stderr y parsea stdout con jc.
func (s *ServerAdapter) ExecuteCommand(ctx context.Context, req *remote_process.ExecuteCommandRequest) (*remote_process.ExecuteCommandResponse, error) {
	// 1. Validaciones iniciales
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "Request cannot be nil")
	}

	// Validar que al menos hay un comando o análisis
	if len(req.Command) == 0 {
		// Si no hay comando, debe haber opciones de análisis válidas
		if req.AnalysisOptions == nil {
			return createErrorResponse("No command or analysis options specified", -1)
		}
		if err := validateAnalysisOptions(req.AnalysisOptions); err != nil {
			return createErrorResponse(fmt.Sprintf("Invalid analysis options: %v", err), -1)
		}
		// En modo classic necesitamos entrada, así que no es válido sin comando
		if req.AnalysisOptions.ParseMode == "classic" {
			return createErrorResponse("Classic parse mode requires command output as input", -1)
		}
		// En modo magic, ejecutamos directamente jc con los argumentos
		return executeJcCommand(ctx, req.AnalysisOptions.Arguments)
	}

	// 2. Validar el directorio de trabajo si está especificado
	if req.WorkingDirectory != "" {
		if _, err := os.Stat(req.WorkingDirectory); err != nil {
			return createErrorResponse(fmt.Sprintf("Invalid working directory: %v", err), -1)
		}
	}

	// 3. Validar variables de entorno
	for k, v := range req.Environment {
		if k == "" {
			return createErrorResponse("Environment variable key cannot be empty", -1)
		}
		if v == "" {
			return createErrorResponse(fmt.Sprintf("Environment variable value cannot be empty for key: %s", k), -1)
		}
	}

	// 4. Validar opciones de análisis si existen
	if req.AnalysisOptions != nil {
		if err := validateAnalysisOptions(req.AnalysisOptions); err != nil {
			return createErrorResponse(fmt.Sprintf("Invalid analysis options: %v", err), -1)
		}
	}

	// 5. Preparar y ejecutar el comando
	cmd := exec.CommandContext(ctx, req.Command[0], req.Command[1:]...)
	cmd.Dir = req.WorkingDirectory

	// Variables de entorno
	env := os.Environ()
	for k, v := range req.Environment {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Capturar stdout y stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Ejecutar el comando
	cmdErr := cmd.Run()
	exitCode := 0
	success := true
	var errorDetails map[string]interface{}

	if cmdErr != nil {
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
			success = false
			errorDetails = map[string]interface{}{
				"type":      "command_error",
				"exit_code": exitCode,
				"error":     cmdErr.Error(),
				"stderr":    stderr.String(),
				"stdout":    stdout.String(),
			}
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to execute command: %v", cmdErr)
		}
	}

	// 6. Procesar la salida estructurada si se solicitó
	var structured *structpb.Struct
	if req.AnalysisOptions != nil && req.AnalysisOptions.ParseMode != "" && stdout.Len() > 0 {
		parseMode := req.AnalysisOptions.GetParseMode()
		parseArgs := req.AnalysisOptions.GetArguments()

		var parseErr error
		switch parseMode {
		case "magic":
			structured, parseErr = parseWithMagicSyntax(parseArgs)
		case "classic":
			structured, parseErr = parseWithClassicSyntax(parseArgs, stdout.Bytes())
		default:
			parseErr = fmt.Errorf("unsupported parse mode: %s", parseMode)
		}

		if parseErr != nil {
			success = false
			errorDetails = map[string]interface{}{
				"type":        "parse_error",
				"parse_mode":  parseMode,
				"error":       parseErr.Error(),
				"exit_code":   exitCode,
				"stderr":      stderr.String(),
				"stdout":      stdout.String(),
				"stdout_size": stdout.Len(),
			}
		}
	}

	// Convertir errorDetails a JSON si existe
	var errorOutput string
	if errorDetails != nil {
		errorJSON, err := json.Marshal(errorDetails)
		if err != nil {
			log.Printf("Error marshaling error details: %v", err)
			errorOutput = fmt.Sprintf("Error interno: %v", err)
		} else {
			errorOutput = string(errorJSON)
		}
	}

	return &remote_process.ExecuteCommandResponse{
		Success:            success,
		ExitCode:           int32(exitCode),
		ErrorOutput:        errorOutput,
		StructuredAnalysis: structured,
	}, nil
}

// createErrorResponse crea una respuesta de error estructurada
func createErrorResponse(message string, exitCode int) (*remote_process.ExecuteCommandResponse, error) {
	errorDetails := map[string]interface{}{
		"type":      "validation_error",
		"error":     message,
		"exit_code": exitCode,
	}

	errorJSON, err := json.Marshal(errorDetails)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Error creating error response: %v", err)
	}

	return &remote_process.ExecuteCommandResponse{
		Success:     false,
		ExitCode:    int32(exitCode),
		ErrorOutput: string(errorJSON),
	}, nil
}

// executeJcCommand ejecuta jc directamente con los argumentos proporcionados
func executeJcCommand(ctx context.Context, args []string) (*remote_process.ExecuteCommandResponse, error) {
	structured, err := parseWithMagicSyntax(args)
	if err != nil {
		errorDetails := map[string]interface{}{
			"type":       "jc_error",
			"error":      err.Error(),
			"arguments":  args,
			"exit_code": -1,
		}

		errorJSON, jsonErr := json.Marshal(errorDetails)
		if jsonErr != nil {
			return nil, status.Errorf(codes.Internal, "Error creating error response: %v", jsonErr)
		}

		return &remote_process.ExecuteCommandResponse{
			Success:     false,
			ExitCode:    -1,
			ErrorOutput: string(errorJSON),
		}, nil
	}

	return &remote_process.ExecuteCommandResponse{
		Success:            true,
		ExitCode:           0,
		StructuredAnalysis: structured,
	}, nil
}

// validateAnalysisOptions valida las opciones de análisis
func validateAnalysisOptions(opts *remote_process.StructuredAnalysisOptions) error {
	if opts.ParseMode != "" && opts.ParseMode != "magic" && opts.ParseMode != "classic" {
		return fmt.Errorf("invalid parse_mode: %s (must be 'magic' or 'classic')", opts.ParseMode)
	}
	if len(opts.Arguments) == 0 {
		return fmt.Errorf("analysis arguments cannot be empty when parse_mode is set")
	}
	return nil
}

// parseWithMagicSyntax ejecuta jc en modo "mágico", ej: `jc ifconfig eth0`
func parseWithMagicSyntax(args []string) (*structpb.Struct, error) {
	cmd := exec.Command("jc", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			switch {
			case code < 100:
				return nil, fmt.Errorf("command error (exit=%d): %s", code, stderr.String())
			case code == 100:
				return nil, fmt.Errorf("jc internal error: %s", stderr.String())
			default:
				cmdCode := code - 100
				return nil, fmt.Errorf("combined error (jc=100, cmd=%d): %s", cmdCode, stderr.String())
			}
		}
		return nil, fmt.Errorf("execution error: %v", err)
	}

	return convertJSONToStruct(stdout.Bytes())
}

// parseWithClassicSyntax usa la forma `jc --<parser>` con stdout como stdin
func parseWithClassicSyntax(args []string, input []byte) (*structpb.Struct, error) {
	cmd := exec.Command("jc", args...)
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code == 100 {
				return nil, fmt.Errorf("jc parsing error: %s", stderr.String())
			}
			return nil, fmt.Errorf("jc error (exit=%d): %s", code, stderr.String())
		}
		return nil, fmt.Errorf("execution error: %v", err)
	}

	return convertJSONToStruct(stdout.Bytes())
}

// convertJSONToStruct ayuda a mapear []byte (JSON) a google.protobuf.Struct
func convertJSONToStruct(jsonData []byte) (*structpb.Struct, error) {
	var data interface{}
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return nil, fmt.Errorf("error unmarshal json: %w", err)
	}

	val, err := structpb.NewValue(data)
	if err != nil {
		return nil, fmt.Errorf("error creando structpb.Value: %w", err)
	}

	// Si es objeto => struct_value. Si es array => list_value => lo envolvemos
	if sVal, ok := val.GetKind().(*structpb.Value_StructValue); ok {
		return sVal.StructValue, nil
	}
	// Envolver la lista/valor primitivo en un struct con campo "items"
	wrap := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"items": val,
		},
	}
	return wrap, nil
}
