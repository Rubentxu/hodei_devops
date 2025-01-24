package api

import (
	"context"
	"log"
	"strings"
	"time"

	"dev.rubentxu.devops-platform/remote_process/internal/domain"
	"dev.rubentxu.devops-platform/remote_process/internal/ports"
)

type MetricsService struct {
	collector ports.MetricsCollector
}

func NewMetricsService(collector ports.MetricsCollector) *MetricsService {
	return &MetricsService{
		collector: collector,
	}
}

func (s *MetricsService) StreamMetrics(ctx context.Context, workerID string, metricTypes []string, interval int64) (<-chan domain.WorkerMetrics, error) {
	metricsChan := make(chan domain.WorkerMetrics)
	log.Printf("Iniciando la recolección de métricas para el worker %s", workerID)
	go func() {
		defer close(metricsChan)
		ticker := time.NewTicker(time.Duration(interval) * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				metrics := domain.WorkerMetrics{
					WorkerID:  workerID,
					Timestamp: time.Now().Unix(),
				}

				// Recolectar métricas solicitadas
				for _, metricType := range metricTypes {
					if err := s.collectMetricByType(ctx, metricType, &metrics); err != nil {
						log.Printf("Error collecting %s metrics: %v", metricType, err)
					}
					log.Printf("Métricas recolectadas: %v", metrics)
				}

				select {
				case metricsChan <- metrics:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return metricsChan, nil
}

func (s *MetricsService) collectMetricByType(ctx context.Context, metricType string, metrics *domain.WorkerMetrics) error {
	var err error
	log.Printf("Recolectando métricas de tipo: %s", metricType)
	switch strings.ToLower(metricType) {
	case "cpu":
		metrics.CPU, err = s.collector.CollectCPUMetrics(ctx)
	case "memory":
		metrics.Memory, err = s.collector.CollectMemoryMetrics(ctx)
	case "disk":
		metrics.Disks, err = s.collector.CollectDiskMetrics(ctx)
	case "network":
		metrics.Networks, err = s.collector.CollectNetworkMetrics(ctx)
	case "system":
		metrics.System, err = s.collector.CollectSystemMetrics(ctx)
	case "process":
		metrics.Processes, err = s.collector.CollectProcessMetrics(ctx)
	case "io":
		metrics.IO, err = s.collector.CollectIOMetrics(ctx)
	default:
		log.Printf("Warning: unknown metric type: %s", metricType)
		return nil
	}

	if err != nil {
		log.Printf("Error collecting %s metrics: %v", metricType, err)
		return nil // Continuar con otras métricas en caso de error
	}

	return err
}

// GetMetricsSnapshot obtiene una instantánea de todas las métricas
func (s *MetricsService) GetMetricsSnapshot(ctx context.Context, workerID string) (*domain.WorkerMetrics, error) {
	metrics := &domain.WorkerMetrics{
		WorkerID:  workerID,
		Timestamp: time.Now().Unix(),
	}

	// Recolectar todas las métricas en paralelo
	errChan := make(chan error, 7)

	go func() {
		var err error
		metrics.CPU, err = s.collector.CollectCPUMetrics(ctx)
		errChan <- err
	}()

	go func() {
		var err error
		metrics.Memory, err = s.collector.CollectMemoryMetrics(ctx)
		errChan <- err
	}()

	go func() {
		var err error
		metrics.Disks, err = s.collector.CollectDiskMetrics(ctx)
		errChan <- err
	}()

	go func() {
		var err error
		metrics.Networks, err = s.collector.CollectNetworkMetrics(ctx)
		errChan <- err
	}()

	go func() {
		var err error
		metrics.System, err = s.collector.CollectSystemMetrics(ctx)
		errChan <- err
	}()

	go func() {
		var err error
		metrics.Processes, err = s.collector.CollectProcessMetrics(ctx)
		errChan <- err
	}()

	go func() {
		var err error
		metrics.IO, err = s.collector.CollectIOMetrics(ctx)
		errChan <- err
	}()

	// Esperar por todos los resultados
	for i := 0; i < 7; i++ {
		if err := <-errChan; err != nil {
			log.Printf("Error collecting metrics: %v", err)
		}
	}

	return metrics, nil
}

// GetSpecificMetrics obtiene métricas específicas
func (s *MetricsService) GetSpecificMetrics(ctx context.Context, workerID string, metricTypes []string) (*domain.WorkerMetrics, error) {
	metrics := &domain.WorkerMetrics{
		WorkerID:  workerID,
		Timestamp: time.Now().Unix(),
	}

	for _, metricType := range metricTypes {
		if err := s.collectMetricByType(ctx, metricType, metrics); err != nil {
			log.Printf("Error collecting %s metrics: %v", metricType, err)
		}
	}

	return metrics, nil
}
