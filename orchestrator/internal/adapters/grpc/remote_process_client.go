package grpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"io/ioutil"
	"log"

	"dev.rubentxu.devops-platform/protos/remote_process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Client encapsulates the gRPC client functionality for RemoteProcess
type Client struct {
	client    remote_process.RemoteProcessServiceClient
	conn      *grpc.ClientConn
	jwtToken  string
	tlsConfig *tls.Config
}

// ClientConfig contiene la configuración necesaria para el cliente
type ClientConfig struct {
	ServerAddress string
	ClientCert    string
	ClientKey     string
	CACert        string
	JWTToken      string
}

// New creates a new instance of the client
func New(cfg *ClientConfig) (*Client, error) {
	// Cargar certificado del cliente
	certificate, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %v", err)
	}

	// Cargar CA cert
	caCert, err := ioutil.ReadFile(cfg.CACert)
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
	conn, err := grpc.Dial(
		cfg.ServerAddress,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}

	client := remote_process.NewRemoteProcessServiceClient(conn)
	return &Client{
		client:    client,
		conn:      conn,
		jwtToken:  cfg.JWTToken,
		tlsConfig: tlsConfig,
	}, nil
}

// createAuthContext crea un contexto con el token JWT
func (c *Client) createAuthContext(ctx context.Context) context.Context {
	md := metadata.New(map[string]string{
		"authorization": "bearer " + c.jwtToken,
	})
	return metadata.NewOutgoingContext(ctx, md)
}

// StartProcess envía una solicitud para iniciar un proceso en el servidor
func (c *Client) StartProcess(ctx context.Context, processID string, command []string, env map[string]string, workingDir string, outputChan chan<- *remote_process.ProcessOutput) error {
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

		select {
		case outputChan <- output:
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// StopProcess envía una solicitud para detener un proceso
func (c *Client) StopProcess(ctx context.Context, processID string) (bool, string, error) {
	ctx = c.createAuthContext(ctx)

	resp, err := c.client.StopProcess(ctx, &remote_process.ProcessStopRequest{
		ProcessId: processID,
	})
	if err != nil {
		return false, "", fmt.Errorf("error stopping process: %v", err)
	}

	return resp.Success, resp.Message, nil
}

// MonitorHealth monitoriza el estado de salud de un proceso
func (c *Client) MonitorHealth(ctx context.Context, processID string, checkInterval int64, healthChan chan<- *remote_process.HealthStatus) error {
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
					case healthChan <- &remote_process.HealthStatus{
						ProcessId: processID,
						IsRunning: false,
						Status:    fmt.Sprintf("Error receiving health status: %v", err),
					}:
					case <-ctx.Done():
					}
					return
				}

				// Enviar el estado al canal
				select {
				case healthChan <- healthStatus:
					log.Printf("Health status for process %s: running=%v, status=%s",
						healthStatus.ProcessId, healthStatus.IsRunning, healthStatus.Status)
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

// Close cierra la conexión gRPC
func (c *Client) Close() {
	if err := c.conn.Close(); err != nil {
		log.Printf("Error closing connection: %v", err)
	}
}
