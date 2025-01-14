package grpc

import (
	"context"
	"dev.rubentxu.devops-platform/protos/remote_process"
	"dev.rubentxu.devops-platform/remote_process/internal/adapters/grpc/security"
	"dev.rubentxu.devops-platform/remote_process/internal/ports"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"crypto/tls"
	"log"
	"net"
)

// ServerAdapter implementa la interfaz RemoteProcessServiceServer generada por protoc
type ServerAdapter struct {
	remote_process.UnimplementedRemoteProcessServiceServer
	executor ports.ProcessExecutor
	port     string
	server   *grpc.Server
}

func NewAdapter(executor ports.ProcessExecutor, port string) *ServerAdapter {
	return &ServerAdapter{
		executor: executor,
		port:     port,
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

	// Utiliza un canal para recibir HealthStatus del método MonitorHealth
	healthChan, err := s.executor.MonitorHealth(stream.Context(), processID, checkInterval)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to start health monitoring: %v", err)
	}

	// Transmite actualizaciones de HealthStatus al cliente
	for healthStatus := range healthChan {
		err := stream.Send(&remote_process.HealthStatus{
			ProcessId: healthStatus.ProcessID,
			IsRunning: healthStatus.IsRunning,
			Status:    healthStatus.Status,
		})
		if err != nil {
			return status.Errorf(codes.Internal, "failed to send health status: %v", err)
		}
		log.Printf("Health status: %v", healthStatus) // Log the health status
	}

	return nil
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

	grpcServer := grpc.NewServer(opts...)
	s.server = grpcServer

	// Registrar servicios de negocio gRPC
	remote_process.RegisterRemoteProcessServiceServer(grpcServer, s)

	// Registrar el servicio de reflexión en entorno de desarrollo
	if env == "development" {
		reflection.Register(grpcServer)
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

func (a *ServerAdapter) Stop() {
	a.server.Stop()
}
