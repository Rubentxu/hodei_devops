package grpc

import (
	"context"
	"google.golang.org/grpc/credentials"

	"dev.rubentxu.devops-platform/protos/remote_process"
	"fmt"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
)

// Client encapsulates the gRPC client functionality for RemoteProcess
type Client struct {
	client remote_process.RemoteProcessServiceClient
	conn   *grpc.ClientConn
}

// New creates a new instance of the client
func New(serverAddress string, creds credentials.TransportCredentials) (*Client, error) {
	conn, err := grpc.NewClient(
		serverAddress,
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}

	client := remote_process.NewRemoteProcessServiceClient(conn)
	return &Client{client: client, conn: conn}, nil
}

// StartProcess sends a request to start a process on the server and receives the output via a channel
func (c *Client) StartProcess(ctx context.Context, processID string, command []string, env map[string]string, workingDir string, outputChan chan<- *remote_process.ProcessOutput) error {
	// Create the stream
	stream, err := c.client.StartProcess(ctx)
	if err != nil {
		return fmt.Errorf("error creating stream: %v", err)
	}

	// Send the initial request
	err = stream.Send(&remote_process.ProcessStartRequest{
		ProcessId:        processID,
		Command:          command,
		Environment:      env,
		WorkingDirectory: workingDir,
	})
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}

	// Close the send stream
	if err := stream.CloseSend(); err != nil {
		return fmt.Errorf("error closing send stream: %v", err)
	}

	// Process responses in a goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in StartProcess: %v", r)
			}
			close(outputChan)
		}()
		for {
			resp, err := stream.Recv()
			if err == io.EOF {
				return // End of stream
			}
			if err != nil {
				log.Printf("Error receiving response: %v", err)
				return
			}

			// Send the output to the channel
			outputChan <- resp
		}
	}()

	return nil
}

// StopProcess sends a request to stop a process on the server
func (c *Client) StopProcess(ctx context.Context, processID string) (bool, string, error) {
	request := &remote_process.ProcessStopRequest{
		ProcessId: processID,
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	response, err := c.client.StopProcess(ctx, request)
	if err != nil {
		return false, "", fmt.Errorf("error stopping process: %v", err)
	}

	return response.Success, response.Message, nil
}

// MonitorHealth inicia el monitoreo de la salud de un proceso
func (c *Client) MonitorHealth(ctx context.Context, processID string, checkInterval int64, healthChan chan<- *remote_process.HealthStatus) error {
	stream, err := c.client.MonitorHealth(ctx)
	if err != nil {
		return fmt.Errorf("error creating stream: %v", err)
	}

	// Enviar la solicitud inicial
	err = stream.Send(&remote_process.HealthCheckRequest{
		ProcessId:     processID,
		CheckInterval: checkInterval,
	})
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}

	// Procesar respuestas en una goroutine
	go func() {
		// Usar un defer recover para manejar posibles pánicos
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered in MonitorHealth: %v", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				// El contexto fue cancelado, salir limpiamente
				return
			default:
				resp, err := stream.Recv()
				if err == io.EOF {
					return
				}
				if err != nil {
					if ctx.Err() == context.Canceled {
						// Contexto cancelado, salir silenciosamente
						return
					}
					log.Printf("Error receiving response: %v", err)
					select {
					case healthChan <- &remote_process.HealthStatus{
						ProcessId: processID,
						IsRunning: false,
						Status:    fmt.Sprintf("Error receiving response: %v", err),
					}:
					case <-ctx.Done():
					}
					return
				}

				// Enviar el estado de salud al canal
				select {
				case healthChan <- resp:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return nil
}

// Close closes the gRPC connection
func (c *Client) Close() {
	if err := c.conn.Close(); err != nil {
		log.Printf("Error closing connection: %v", err)
	}
}
