package ports

import (
	"context"

	"dev.rubentxu.devops-platform/remote_process/internal/domain"
)

// MetricsService define el contrato para el servicio de métricas
type MetricsService interface {
	StreamMetrics(ctx context.Context, workerID string, metricTypes []string, interval int64) (<-chan domain.WorkerMetrics, error)
	GetMetricsSnapshot(ctx context.Context, workerID string) (*domain.WorkerMetrics, error)
	GetSpecificMetrics(ctx context.Context, workerID string, metricTypes []string) (*domain.WorkerMetrics, error)
}

// MetricsCollector define el contrato para la recolección de métricas del sistema
type MetricsCollector interface {
	CollectCPUMetrics(ctx context.Context) (*domain.CPUMetrics, error)
	CollectMemoryMetrics(ctx context.Context) (*domain.MemoryMetrics, error)
	CollectDiskMetrics(ctx context.Context) ([]domain.DiskMetrics, error)
	CollectNetworkMetrics(ctx context.Context) ([]domain.NetworkMetrics, error)
	CollectSystemMetrics(ctx context.Context) (*domain.SystemMetrics, error)
	CollectProcessMetrics(ctx context.Context) ([]domain.ProcessMetrics, error)
	CollectIOMetrics(ctx context.Context) (*domain.IOMetrics, error)
}
