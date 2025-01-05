package remote_process_server

import (
	"context"
	pb "dev.rubentxu.devops-platform/adapters/grpc/protos/remote_process"
	"dev.rubentxu.devops-platform/domain/ports"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"log"
	"net"
	"sync"
)

// server implementa la interfaz RemoteProcessServiceServer generada por protoc
type server struct {
	pb.UnimplementedRemoteProcessServiceServer
	executor ports.ProcessExecutor
	mu       sync.Mutex
}

// New crea una nueva instancia del servidor gRPC
func New(executor ports.ProcessExecutor) *grpc.Server {
	srv := &server{
		executor: executor,
	}

	grpcServer := grpc.NewServer()
	pb.RegisterRemoteProcessServiceServer(grpcServer, srv)
	return grpcServer
}

// StartProcess implementa la lógica para iniciar un proceso con streaming de salida
func (s *server) StartProcess(stream pb.RemoteProcessService_StartProcessServer) error {
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
		err := stream.Send(&pb.ProcessOutput{
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
func (s *server) StopProcess(ctx context.Context, req *pb.ProcessStopRequest) (*pb.ProcessStopResponse, error) {
	err := s.executor.Stop(ctx, req.ProcessId)
	if err != nil {
		log.Printf("Error stopping process: %v", err)
		return nil, status.Errorf(codes.Internal, "failed to stop process: %v", err)
	}

	return &pb.ProcessStopResponse{Success: true, Message: "Process stopped successfully"}, nil
}

// MonitorHealth implementa la lógica para monitorizar el estado de un proceso
func (s *server) MonitorHealth(stream pb.RemoteProcessService_MonitorHealthServer) error {
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
		err := stream.Send(&pb.HealthStatus{
			ProcessId: healthStatus.ProcessID,
			IsRunning: healthStatus.IsRunning,
			Status:    healthStatus.Status,
		})
		if err != nil {
			return status.Errorf(codes.Internal, "failed to send health status: %v", err)
		}
	}

	return nil
}

// Start inicia el servidor gRPC
func Start(address string, executor ports.ProcessExecutor) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	grpcServer := New(executor)

	log.Printf("Starting gRPC server on %s...", address)
	if err := grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}
