package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"dev.rubentxu.devops-platform/worker/internal/domain"
	"fmt"
	"io"
	"log"
	"os"

	"dev.rubentxu.devops-platform/protos/remote_process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// RPSClient encapsulates the gRPC client functionality for RemoteProcess
type RPSClient struct {
	client    remote_process.RemoteProcessServiceClient
	conn      *grpc.ClientConn
	jwtToken  string
	tlsConfig *tls.Config
}

// RemoteProcessClientConfig contiene la configuración necesaria para el cliente
type RemoteProcessClientConfig struct {
	Address    string
	ClientCert string
	ClientKey  string
	CACert     string
	AuthToken  string
}

// New creates a new instance of the client
func New(cfg *RemoteProcessClientConfig) (*RPSClient, error) {
	// Cargar certificado del cliente
	certificate, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %v", err)
	}

	// Cargar CA cert
	caCert, err := os.ReadFile(cfg.CACert)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to append CA certificate")
	}

	// Configurar TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{certificate},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
	}

	// Establecer conexión con credenciales TLS
	conn, err := grpc.NewClient(
		cfg.Address,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}

	client := remote_process.NewRemoteProcessServiceClient(conn)
	return &RPSClient{
		client:    client,
		conn:      conn,
		jwtToken:  cfg.AuthToken,
		tlsConfig: tlsConfig,
	}, nil
}

// createAuthContext crea un contexto con el token JWT
func (c *RPSClient) createAuthContext(ctx context.Context) context.Context {
	md := metadata.New(map[string]string{
		"authorization": "bearer " + c.jwtToken,
	})
	return metadata.NewOutgoingContext(ctx, md)
}

// StartProcess envía una solicitud para iniciar un proceso en el servidor
func (c *RPSClient) StartProcess(ctx context.Context, processID string, command []string, env map[string]string, workingDir string, outputChan chan<- *domain.ProcessOutput) error {
	ctx = c.createAuthContext(ctx)

	// Crear el stream
	stream, err := c.client.StartProcess(ctx)
	if err != nil {
		return fmt.Errorf("error creating stream: %v", err)
	}

	// Enviar la solicitud inicial
	err = stream.Send(&remote_process.ProcessStartRequest{
		ProcessId:        processID,
		Command:          command,
		Environment:      env,
		WorkingDirectory: workingDir,
	})
	if err != nil {
		return fmt.Errorf("error sending initial request: %v", err)
	}

	// Recibir la salida del proceso
	for {
		output, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error receiving output: %v", err)
		}

		// Convertir remote_process.ProcessOutput a domain.ProcessOutput
		domainOutput := &domain.ProcessOutput{
			ProcessID: output.ProcessId,
			Output:    output.Output,
			IsError:   output.IsError,
		}

		select {
		case outputChan <- domainOutput:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// StopProcess envía una solicitud para detener un proceso
func (c *RPSClient) StopProcess(ctx context.Context, processID string) (bool, string, error) {
	ctx = c.createAuthContext(ctx)

	resp, err := c.client.StopProcess(ctx, &remote_process.ProcessStopRequest{
		ProcessId: processID,
	})
	if err != nil {
		return false, "", fmt.Errorf("error stopping process: %v", err)
	}

	return resp.Success, resp.Message, nil
}

// / MonitorHealth monitoriza el estado de salud de un proceso
func (c *RPSClient) MonitorHealth(ctx context.Context, processID string, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error {
	ctx = c.createAuthContext(ctx)

	stream, err := c.client.MonitorHealth(ctx)
	if err != nil {
		return fmt.Errorf("error creating health monitor stream: %v", err)
	}

	// Enviar solicitud inicial de monitoreo
	err = stream.Send(&remote_process.HealthCheckRequest{
		ProcessId:     processID,
		CheckInterval: checkInterval,
	})
	if err != nil {
		return fmt.Errorf("error sending health check request: %v", err)
	}

	// Procesar respuestas en una goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in MonitorHealth: %v", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			default:
				healthStatus, err := stream.Recv()
				if err == io.EOF {
					return
				}
				if err != nil {
					if ctx.Err() == context.Canceled {
						return
					}
					log.Printf("Error receiving health status: %v", err)
					select {
					case healthChan <- &domain.ProcessHealthStatus{
						ProcessID: processID,
						Status:    domain.ERROR,
						Message:   fmt.Sprintf("Error receiving health status: %v", err),
					}:
					case <-ctx.Done():
					}
					return
				}

				// Enviar el estado al canal
				select {
				case healthChan <- &domain.ProcessHealthStatus{
					ProcessID: processID,
					Status:    domain.ConvertProtoProcessStatusToPorts(healthStatus.Status),
					Message:   healthStatus.Message,
				}:
					log.Printf("Health status for process %s: status=%v, message=%s",
						healthStatus.ProcessId, healthStatus.Status, healthStatus.Message)
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

// StreamMetrics envía una solicitud para obtener métricas de un proceso
func (c *RPSClient) StreamMetrics(ctx context.Context, workerID string, metricTypes []string, interval int64, metricsChan chan<- *remote_process.WorkerMetrics) error {
	ctx = c.createAuthContext(ctx)

	stream, err := c.client.CollectMetrics(ctx)
	if err != nil {
		return fmt.Errorf("error creating metrics stream: %w", err)
	}

	// Enviar solicitud inicial
	err = stream.Send(&remote_process.MetricsRequest{
		WorkerId:    workerID,
		MetricTypes: metricTypes,
		Interval:    interval,
	})
	if err != nil {
		return fmt.Errorf("error sending metrics request: %w", err)
	}

	// Recibir métricas
	for {
		metrics, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("error receiving metrics: %w", err)
		}

		select {
		case metricsChan <- metrics:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// Close cierra la conexión gRPC
func (c *RPSClient) Close() {
	if err := c.conn.Close(); err != nil {
		log.Printf("Error closing connection: %v", err)
	}
}
