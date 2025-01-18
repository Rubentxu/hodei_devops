package ports

import (
	"context"
	"dev.rubentxu.devops-platform/worker/internal/domain"
)

type WorkerInstance interface {
	Start(ctx context.Context) (*domain.WorkerID, error)
	Run(ctx context.Context, t domain.Task, outputChan chan<- *domain.ProcessOutput) domain.TaskResult
	Stop() (bool, string, error)
	StartMonitoring(ctx context.Context, checkInterval int64, healthChan chan<- *domain.ProcessHealthStatus) error
}
