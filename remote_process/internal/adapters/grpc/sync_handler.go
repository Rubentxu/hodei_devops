package grpc

import (
	"context"
	"fmt"
	"io"

	"dev.rubentxu.devops-platform/protos/remote_process"
	"dev.rubentxu.devops-platform/remote_process/internal/domain"
	"dev.rubentxu.devops-platform/remote_process/internal/ports"
)

// SyncHandler maneja las operaciones de sincronización a través de gRPC
type SyncHandler struct {
	syncService ports.SyncService
}

// NewSyncHandler crea una nueva instancia del manejador de sincronización
func NewSyncHandler(service ports.SyncService) *SyncHandler {
	return &SyncHandler{
		syncService: service,
	}
}

func (h *SyncHandler) SyncFiles(req *remote_process.FileSyncRequest, stream remote_process.RemoteProcessService_SyncFilesServer) error {
	// Convertir la solicitud del proto al dominio
	domainRequest := domain.FileSyncRequest{
		SourcePath:      req.SourcePath,
		DestinationPath: req.DestinationPath,
		Options:         convertProtoOptionsToSyncOptions(req.Options),
	}

	// Iniciar la sincronización
	progressChan, err := h.syncService.SyncFiles(stream.Context(), domainRequest)
	if err != nil {
		return err
	}

	// Manejar el progreso
	for progress := range progressChan {
		// Convertir el progreso a proto y enviarlo
		if err := stream.Send(convertToProtoProgress(progress)); err != nil {
			return fmt.Errorf("error sending progress: %w", err)
		}
	}

	return nil
}

func (h *SyncHandler) SyncFilesStream(stream remote_process.RemoteProcessService_SyncFilesStreamServer) error {
	var currentUpload *domain.FileUpload

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("error receiving file data: %w", err)
		}

		switch r := req.Request.(type) {
		case *remote_process.FileUploadRequest_Metadata:
			metadata := convertProtoMetadataToDomain(r.Metadata)
			currentUpload, err = h.syncService.InitiateFileUpload(stream.Context(), metadata)
			if err != nil {
				return err
			}

		case *remote_process.FileUploadRequest_Chunk:
			if currentUpload == nil {
				return fmt.Errorf("received chunk without metadata")
			}

			chunk := convertProtoChunkToDomain(r.Chunk)
			progress, err := h.syncService.ProcessFileChunk(currentUpload, chunk)
			if err != nil {
				return err
			}

			// Enviar progreso
			if err := stream.Send(convertToProtoProgress(progress)); err != nil {
				return fmt.Errorf("error sending progress: %w", err)
			}
		}
	}
}

func convertToProtoProgress(progress domain.SyncProgress) *remote_process.SyncProgress {
	return &remote_process.SyncProgress{
		FilePath:         progress.FilePath,
		BytesTransferred: progress.BytesTransferred,
		TotalBytes:       progress.TotalBytes,
		Status: &remote_process.SyncStatus{
			Action:  convertSyncAction(progress.Status.Action),
			Details: progress.Status.Details,
		},
		ErrorMessage:       progress.ErrorMessage,
		ProgressPercentage: progress.ProgressPercent,
		Stats:              convertSyncStats(progress.Stats),
	}
}

func convertSyncAction(action domain.SyncAction) remote_process.SyncStatus_Action {
	switch action {
	case domain.Unknown:
		return remote_process.SyncStatus_UNKNOWN
	case domain.Scanning:
		return remote_process.SyncStatus_SCANNING
	case domain.Comparing:
		return remote_process.SyncStatus_COMPARING
	case domain.Copying:
		return remote_process.SyncStatus_COPYING
	case domain.Deleting:
		return remote_process.SyncStatus_DELETING
	case domain.Linking:
		return remote_process.SyncStatus_LINKING
	case domain.Skipping:
		return remote_process.SyncStatus_SKIPPING
	case domain.Verifying:
		return remote_process.SyncStatus_VERIFYING
	case domain.Done:
		return remote_process.SyncStatus_DONE
	case domain.Error:
		return remote_process.SyncStatus_ERROR
	default:
		return remote_process.SyncStatus_UNKNOWN
	}
}

func convertSyncStats(stats domain.SyncStats) *remote_process.SyncStats {
	return &remote_process.SyncStats{
		FilesTransferred: stats.FilesTransferred,
		FilesDeleted:     stats.FilesDeleted,
		FilesSkipped:     stats.FilesSkipped,
		TotalFiles:       stats.TotalFiles,
		TotalSize:        stats.TotalSize,
		TransferSpeed:    stats.TransferSpeed,
		Errors:           stats.Errors,
		ErrorList:        stats.ErrorList,
	}
}

// GetRemoteFileList maneja las solicitudes para listar archivos remotos
func (h *SyncHandler) GetRemoteFileList(ctx context.Context, req *remote_process.RemoteListRequest) (*remote_process.RemoteFileList, error) {
	files, err := h.syncService.ListFiles(ctx, domain.ListFilesRequest{
		Path:      req.Path,
		Recursive: req.Recursive,
		Include:   req.Include,
		Exclude:   req.Exclude,
	})
	if err != nil {
		return nil, fmt.Errorf("error listing files: %w", err)
	}

	response := &remote_process.RemoteFileList{
		Files: make([]*remote_process.RemoteFileInfo, len(files)),
	}

	for i, file := range files {
		response.Files[i] = convertFileMetadataToProto(file)
	}

	return response, nil
}

// DeleteRemoteFile maneja las solicitudes para eliminar archivos remotos
func (h *SyncHandler) DeleteRemoteFile(ctx context.Context, req *remote_process.RemoteDeleteRequest) (*remote_process.RemoteDeleteResponse, error) {
	err := h.syncService.DeleteFile(ctx, domain.DeleteFileRequest{
		Path:      req.Path,
		Recursive: req.Recursive,
		Force:     req.Force,
	})

	if err != nil {
		return &remote_process.RemoteDeleteResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &remote_process.RemoteDeleteResponse{
		Success: true,
		Message: "File deleted successfully",
	}, nil
}

// Funciones auxiliares de conversión
func convertFileMetadataToProto(metadata *domain.FileMetadata) *remote_process.RemoteFileInfo {
	acls := make([]*remote_process.ACLEntry, len(metadata.ACLs))
	for i, acl := range metadata.ACLs {
		acls[i] = &remote_process.ACLEntry{
			Principal: acl.Principal,
			Perms:     acl.Perms,
			Type:      acl.Type,
		}
	}

	return &remote_process.RemoteFileInfo{
		Path:       metadata.Path,
		Size:       metadata.Size,
		ModTime:    metadata.ModTime.Unix(),
		Mode:       uint32(metadata.Mode),
		IsDir:      metadata.IsDir,
		IsSymlink:  metadata.IsSymlink,
		Owner:      metadata.Owner,
		Group:      metadata.Group,
		Checksum:   metadata.Checksum,
		LinkTarget: metadata.LinkTarget,
		Acls:       acls,
		Xattrs:     metadata.XAttrs,
	}
}

func convertProtoOptionsToSyncOptions(proto *remote_process.SyncOptions) domain.SyncOptions {
	return domain.SyncOptions{
		Recursive:     proto.Recursive,
		Archive:       proto.Archive,
		Compress:      proto.Compress,
		Delete:        proto.Delete,
		DryRun:        proto.DryRun,
		PreservePerms: proto.PreservePerms,
		PreserveTimes: proto.PreserveTimes,
		PreserveOwner: proto.PreserveOwner,
		PreserveGroup: proto.PreserveGroup,
		Checksum:      proto.Checksum,
		UseDelta:      proto.UseDelta,
		BlockSize:     proto.BlockSize,
		// ... otros campos
	}
}

// Funciones de conversión
func convertProtoMetadataToDomain(proto *remote_process.FileSyncMetadata) *domain.FileSyncMetadata {
	return &domain.FileSyncMetadata{
		SourcePath:      proto.SourcePath,
		DestinationPath: proto.DestinationPath,
		TotalSize:       proto.TotalSize,
		Options:         convertProtoOptionsToSyncOptions(proto.Options),
	}
}

func convertProtoChunkToDomain(proto *remote_process.FileChunk) *domain.FileChunk {
	chunk := &domain.FileChunk{
		Offset: proto.Offset,
		IsLast: proto.IsLast,
	}

	switch data := proto.Data.(type) {
	case *remote_process.FileChunk_Content:
		chunk.Content = data.Content
	case *remote_process.FileChunk_Delta:
		chunk.DeltaRef = &domain.DeltaReference{
			SourceOffset: data.Delta.SourceOffset,
			Length:       data.Delta.Length,
			Checksum:     data.Delta.Checksum,
		}
	}

	return chunk
}
