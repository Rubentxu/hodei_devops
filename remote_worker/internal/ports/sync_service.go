package ports

import (
	"context"

	"dev.rubentxu.hodei-devops/remote_worker/internal/domain"
)

// SyncService define el contrato para el servicio de sincronización
type SyncService interface {
	SyncFiles(ctx context.Context, request domain.FileSyncRequest) (<-chan domain.SyncProgress, error)
	SyncDirectory(ctx context.Context, request domain.FileSyncRequest) (<-chan domain.SyncProgress, error)
	ListFiles(ctx context.Context, request domain.ListFilesRequest) ([]*domain.FileMetadata, error)
	DeleteFile(ctx context.Context, request domain.DeleteFileRequest) error
	GetFileMetadata(ctx context.Context, path string) (*domain.FileMetadata, error)
	InitiateFileUpload(ctx context.Context, metadata *domain.FileSyncMetadata) (*domain.FileUpload, error)
	ProcessFileChunk(upload *domain.FileUpload, chunk *domain.FileChunk) (domain.SyncProgress, error)
}
