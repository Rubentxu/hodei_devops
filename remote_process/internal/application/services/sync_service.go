package services

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"dev.rubentxu.devops-platform/remote_process/internal/domain"
)

// SyncService proporciona funcionalidad para sincronizar archivos y directorios
type SyncService struct {
	progressChan chan domain.SyncProgress
	bufferSize   int
}

// NewSyncService crea una nueva instancia del servicio de sincronización
func NewSyncService(bufferSize int) *SyncService {
	if bufferSize <= 0 {
		bufferSize = 32 * 1024 // 32KB por defecto
	}
	return &SyncService{
		bufferSize: bufferSize,
	}
}

// SyncFiles sincroniza archivos entre origen y destino
func (s *SyncService) SyncFiles(ctx context.Context, request domain.FileSyncRequest) (<-chan domain.SyncProgress, error) {
	s.progressChan = make(chan domain.SyncProgress, 100) // Buffer para evitar bloqueos

	go func() {
		defer close(s.progressChan)

		// Validar rutas y opciones
		if err := s.validateRequest(request); err != nil {
			s.sendError(err)
			return
		}

		// Iniciar escaneo
		s.sendProgress(domain.SyncProgress{
			Status: domain.SyncStatus{
				Action:  domain.Scanning,
				Details: "Scanning source directory",
			},
		})

		// Obtener lista de archivos
		fileList, err := s.scanDirectory(request.SourcePath, request.Options)
		if err != nil {
			s.sendError(fmt.Errorf("error scanning directory: %w", err))
			return
		}

		// Iniciar sincronización
		stats := &domain.SyncStats{
			TotalFiles: int64(len(fileList)),
		}

		// Procesar archivos
		for _, file := range fileList {
			select {
			case <-ctx.Done():
				s.sendError(ctx.Err())
				return
			default:
				if err := s.processFile(ctx, file, request, stats); err != nil {
					stats.Errors++
					stats.ErrorList = append(stats.ErrorList, err.Error())
				}
			}
		}

		// Eliminar archivos si se solicita
		if request.Options.Delete {
			if err := s.deleteExtraFiles(request, fileList, stats); err != nil {
				s.sendError(err)
				return
			}
		}

		// Enviar estadísticas finales
		s.sendProgress(domain.SyncProgress{
			Status: domain.SyncStatus{
				Action:  domain.Done,
				Details: "Synchronization completed",
			},
			Stats: *stats,
		})
	}()

	return s.progressChan, nil
}

func (s *SyncService) SyncDirectory(ctx context.Context, request domain.FileSyncRequest) (<-chan domain.SyncProgress, error) {
	s.progressChan = make(chan domain.SyncProgress, 100)

	go func() {
		defer close(s.progressChan)

		// Escanear directorios y obtener metadatos
		sourceFiles, err := s.scanDirectoryWithMetadata(request.SourcePath, request.Options)
		if err != nil {
			s.sendError(err)
			return
		}

		targetFiles, err := s.scanDirectoryWithMetadata(request.DestinationPath, request.Options)
		if err != nil {
			s.sendError(err)
			return
		}

		// Calcular diferencias
		changes := s.calculateChanges(sourceFiles, targetFiles, request.Options)

		// Aplicar cambios
		stats := &domain.SyncStats{}
		for _, change := range changes {
			select {
			case <-ctx.Done():
				return
			default:
				if err := s.applyChange(ctx, change, request.Options, stats); err != nil {
					stats.Errors++
					stats.ErrorList = append(stats.ErrorList, err.Error())
				}
			}
		}

		// Enviar estadísticas finales
		s.sendProgress(domain.SyncProgress{
			Status: domain.SyncStatus{Action: domain.Done},
			Stats:  *stats,
		})
	}()

	return s.progressChan, nil
}

// SECCIÓN 2: Métodos de manejo de archivos individuales
func (s *SyncService) InitiateFileUpload(ctx context.Context, metadata *domain.FileSyncMetadata) (*domain.FileUpload, error) {
	// Crear el directorio destino si no existe
	destDir := filepath.Dir(metadata.DestinationPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("error creating destination directory: %w", err)
	}

	// Abrir el archivo destino
	file, err := os.OpenFile(metadata.DestinationPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("error creating destination file: %w", err)
	}

	// Crear el objeto FileUpload
	upload := &domain.FileUpload{
		Metadata: metadata,
		File:     file,
		Progress: &domain.SyncProgress{
			FilePath:   metadata.DestinationPath,
			TotalBytes: metadata.TotalSize,
			Status: domain.SyncStatus{
				Action:  domain.Copying,
				Details: fmt.Sprintf("Uploading %s", filepath.Base(metadata.SourcePath)),
			},
		},
	}

	return upload, nil
}

func (s *SyncService) ProcessFileChunk(upload *domain.FileUpload, chunk *domain.FileChunk) (domain.SyncProgress, error) {
	if upload == nil || upload.File == nil {
		return domain.SyncProgress{}, fmt.Errorf("invalid upload session")
	}

	var err error
	if chunk.DeltaRef != nil {
		err = s.processDeltaChunk(upload, chunk)
	} else {
		_, err = upload.File.WriteAt(chunk.Content, chunk.Offset)
	}

	if err != nil {
		return domain.SyncProgress{}, fmt.Errorf("error writing chunk: %w", err)
	}

	// Actualizar progreso
	upload.Progress.BytesTransferred += int64(len(chunk.Content))
	if chunk.DeltaRef != nil {
		upload.Progress.BytesTransferred += int64(chunk.DeltaRef.Length)
	}

	// Calcular porcentaje
	if upload.Progress.TotalBytes > 0 {
		upload.Progress.ProgressPercent = float64(upload.Progress.BytesTransferred) / float64(upload.Progress.TotalBytes) * 100
	}

	// Si es el último chunk, cerrar el archivo
	if chunk.IsLast {
		if err := upload.File.Close(); err != nil {
			return domain.SyncProgress{}, fmt.Errorf("error closing file: %w", err)
		}
		upload.Progress.Status.Action = domain.Done
		upload.Progress.Status.Details = "Upload completed"
	}

	// Crear una copia del progreso
	progress := domain.SyncProgress{
		FilePath:         upload.Progress.FilePath,
		BytesTransferred: upload.Progress.BytesTransferred,
		TotalBytes:       upload.Progress.TotalBytes,
		Status:           upload.Progress.Status,
		ErrorMessage:     upload.Progress.ErrorMessage,
		ProgressPercent:  upload.Progress.ProgressPercent,
		Stats:            upload.Progress.Stats,
	}
	return progress, nil
}

// SECCIÓN 3: Métodos de listado y eliminación
func (s *SyncService) ListFiles(ctx context.Context, request domain.ListFilesRequest) ([]*domain.FileMetadata, error) {
	var files []*domain.FileMetadata

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Aplicar filtros de inclusión/exclusión
		if !request.Recursive && path != request.Path && info.IsDir() {
			return filepath.SkipDir
		}

		if s.shouldSkipFile(path, domain.SyncOptions{
			Include: request.Include,
			Exclude: request.Exclude,
		}) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		metadata, err := s.getFileMetadata(path, info)
		if err != nil {
			return err
		}

		files = append(files, metadata)
		return nil
	}

	if err := filepath.Walk(request.Path, walkFn); err != nil {
		return nil, fmt.Errorf("error walking directory: %w", err)
	}

	return files, nil
}

func (s *SyncService) DeleteFile(ctx context.Context, request domain.DeleteFileRequest) error {
	info, err := os.Lstat(request.Path)
	if err != nil {
		if os.IsNotExist(err) && request.Force {
			return nil
		}
		return fmt.Errorf("error getting file info: %w", err)
	}

	if info.IsDir() && !request.Recursive {
		return fmt.Errorf("cannot delete directory without recursive flag")
	}

	if info.IsDir() {
		if err := os.RemoveAll(request.Path); err != nil {
			return fmt.Errorf("error removing directory: %w", err)
		}
	} else {
		if err := os.Remove(request.Path); err != nil {
			return fmt.Errorf("error removing file: %w", err)
		}
	}

	return nil
}

// SECCIÓN 4: Métodos de metadatos y utilidades
func (s *SyncService) GetFileMetadata(ctx context.Context, path string) (*domain.FileMetadata, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("error getting file info: %w", err)
	}

	return s.getFileMetadata(path, info)
}

func (s *SyncService) calculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// SECCIÓN 5: Métodos privados de ayuda
func (s *SyncService) sendProgress(progress domain.SyncProgress) {
	select {
	case s.progressChan <- progress:
	default:
		// Evitar bloqueo si el canal está lleno
	}
}

func (s *SyncService) sendError(err error) {
	s.sendProgress(domain.SyncProgress{
		Status: domain.SyncStatus{
			Action:  domain.Error,
			Details: err.Error(),
		},
		ErrorMessage: err.Error(),
	})
}

func (s *SyncService) validateRequest(request domain.FileSyncRequest) error {
	if request.SourcePath == "" || request.DestinationPath == "" {
		return fmt.Errorf("source and destination paths are required")
	}

	srcInfo, err := os.Stat(request.SourcePath)
	if err != nil {
		return fmt.Errorf("source path error: %w", err)
	}

	if !srcInfo.IsDir() && request.Options.Recursive {
		return fmt.Errorf("source must be a directory when recursive option is enabled")
	}

	if request.Options.CompressLevel < 0 || request.Options.CompressLevel > 9 {
		return fmt.Errorf("invalid compression level: must be between 0 and 9")
	}

	return nil
}

// SECCIÓN 6: Métodos de transferencia delta
func (s *SyncService) processDeltaChunk(upload *domain.FileUpload, chunk *domain.FileChunk) error {
	// Abrir el archivo fuente para leer el bloque referenciado
	sourceFile, err := os.Open(upload.Metadata.SourcePath)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer sourceFile.Close()

	// Leer el bloque referenciado
	buffer := make([]byte, chunk.DeltaRef.Length)
	_, err = sourceFile.ReadAt(buffer, chunk.DeltaRef.SourceOffset)
	if err != nil {
		return fmt.Errorf("error reading source block: %w", err)
	}

	// Verificar checksum si está disponible
	if chunk.DeltaRef.Checksum != "" {
		checksum := s.calculateChecksumFromBuffer(buffer)
		if checksum != chunk.DeltaRef.Checksum {
			return fmt.Errorf("checksum mismatch for delta block")
		}
	}

	// Escribir el bloque en el archivo destino
	_, err = upload.File.WriteAt(buffer, chunk.Offset)
	if err != nil {
		return fmt.Errorf("error writing delta block: %w", err)
	}

	return nil
}

func (s *SyncService) calculateFileBlocks(path string) []domain.DeltaBlock {
	// Implementar cálculo de bloques usando rolling checksum
	return nil
}

func (s *SyncService) findDeltas(sourcePath string, targetBlocks []domain.DeltaBlock) []domain.DeltaBlock {
	// Implementar búsqueda de diferencias
	return nil
}

func (s *SyncService) applyDeltas(ctx context.Context, deltas []domain.DeltaBlock, targetPath string, stats *domain.SyncStats) error {
	// Implementar aplicación de deltas
	return nil
}

// SECCIÓN 7: Métodos de preservación de metadatos
func (s *SyncService) preserveMetadata(src, dest string, srcInfo os.FileInfo, options domain.SyncOptions) error {
	if options.PreservePerms {
		if err := os.Chmod(dest, srcInfo.Mode()); err != nil {
			return err
		}
	}

	if options.PreserveTimes {
		atime := time.Now()
		mtime := srcInfo.ModTime()
		if err := os.Chtimes(dest, atime, mtime); err != nil {
			return err
		}
	}

	if options.PreserveOwner || options.PreserveGroup {
		if err := s.preserveOwnership(src, dest); err != nil {
			return err
		}
	}

	return nil
}

func (s *SyncService) preserveOwnership(src, dest string) error {
	// Esta función requiere privilegios de root en sistemas Unix
	// Implementar según el sistema operativo
	return nil
}

func parseBandwidthLimit(limit string) (int64, error) {
	limit = strings.ToUpper(limit)
	var multiplier int64 = 1

	switch {
	case strings.HasSuffix(limit, "K"):
		multiplier = 1024
		limit = strings.TrimSuffix(limit, "K")
	case strings.HasSuffix(limit, "M"):
		multiplier = 1024 * 1024
		limit = strings.TrimSuffix(limit, "M")
	case strings.HasSuffix(limit, "G"):
		multiplier = 1024 * 1024 * 1024
		limit = strings.TrimSuffix(limit, "G")
	}

	value, err := strconv.ParseInt(limit, 10, 64)
	if err != nil {
		return 0, err
	}

	return value * multiplier, nil
}

func (s *SyncService) scanDirectory(path string, options domain.SyncOptions) ([]string, error) {
	var files []string
	var maxDepth = options.MaxDepth

	err := filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Verificar profundidad máxima
		if maxDepth > 0 {
			relPath, err := filepath.Rel(path, currentPath)
			if err != nil {
				return err
			}
			depth := len(strings.Split(relPath, string(os.PathSeparator)))
			if depth > int(maxDepth) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Saltar directorios
		if info.IsDir() {
			return nil
		}

		// Aplicar filtros
		if s.shouldSkipFile(currentPath, options) {
			return nil
		}

		files = append(files, currentPath)
		return nil
	})

	return files, err
}

func (s *SyncService) scanDirectoryWithMetadata(path string, options domain.SyncOptions) (map[string]*domain.FileMetadata, error) {
	files := make(map[string]*domain.FileMetadata)

	err := filepath.Walk(path, func(currentPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if s.shouldSkipFile(currentPath, options) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		metadata, err := s.getFileMetadata(currentPath, info)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(path, currentPath)
		if err != nil {
			return err
		}

		files[relPath] = metadata
		return nil
	})

	return files, err
}

func (s *SyncService) calculateChanges(source, target map[string]*domain.FileMetadata, options domain.SyncOptions) []domain.FileChange {
	var changes []domain.FileChange

	// Verificar archivos nuevos y modificados
	for path, srcMeta := range source {
		targetMeta, exists := target[path]
		if !exists {
			changes = append(changes, domain.FileChange{
				Path:           path,
				ChangeType:     domain.Create,
				SourceMetadata: srcMeta,
			})
			continue
		}

		if s.needsSync(srcMeta, targetMeta, options) {
			changes = append(changes, domain.FileChange{
				Path:           path,
				ChangeType:     domain.Modify,
				SourceMetadata: srcMeta,
				TargetMetadata: targetMeta,
			})
		}
	}

	// Verificar archivos eliminados
	if options.Delete {
		for path, targetMeta := range target {
			if _, exists := source[path]; !exists {
				changes = append(changes, domain.FileChange{
					Path:           path,
					ChangeType:     domain.Delete,
					TargetMetadata: targetMeta,
				})
			}
		}
	}

	return changes
}

func (s *SyncService) applyChange(ctx context.Context, change domain.FileChange, options domain.SyncOptions, stats *domain.SyncStats) error {
	switch change.ChangeType {
	case domain.Create, domain.Modify:
		if change.SourceMetadata.IsSymlink {
			return s.createSymlink(change.SourceMetadata.LinkTarget, change.Path)
		}
		return s.syncFileWithDelta(ctx, change, options, stats)

	case domain.Delete:
		return s.deleteFile(change.Path, stats)
	}

	return nil
}

func (s *SyncService) syncFileWithDelta(ctx context.Context, change domain.FileChange, options domain.SyncOptions, stats *domain.SyncStats) error {
	if !options.UseDelta || change.TargetMetadata == nil {
		// Sincronización completa si no hay delta o el archivo no existe
		return s.copyFileWithProgress(ctx, change.Path, change.Path, stats, options)
	}

	// Calcular y aplicar deltas
	blocks := s.calculateFileBlocks(change.Path)
	deltas := s.findDeltas(change.SourceMetadata.Path, blocks)

	return s.applyDeltas(ctx, deltas, change.Path, stats)
}

func (s *SyncService) deleteExtraFiles(request domain.FileSyncRequest, sourceFiles []string, stats *domain.SyncStats) error {
	sourceSet := make(map[string]bool)
	for _, file := range sourceFiles {
		rel, err := filepath.Rel(request.SourcePath, file)
		if err != nil {
			continue
		}
		sourceSet[rel] = true
	}

	return filepath.Walk(request.DestinationPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(request.DestinationPath, path)
		if err != nil {
			return err
		}

		if !sourceSet[rel] {
			s.sendProgress(domain.SyncProgress{
				FilePath: rel,
				Status: domain.SyncStatus{
					Action:  domain.Deleting,
					Details: "Deleting extra file",
				},
			})

			if err := os.Remove(path); err != nil {
				return err
			}

			stats.FilesDeleted++
		}

		return nil
	})
}

func (s *SyncService) getFileMetadata(path string, info os.FileInfo) (*domain.FileMetadata, error) {
	metadata := &domain.FileMetadata{
		Path:    path,
		Size:    info.Size(),
		ModTime: info.ModTime(),
		Mode:    info.Mode(),
		IsDir:   info.IsDir(),
	}

	// Verificar si es un symlink
	if info.Mode()&os.ModeSymlink != 0 {
		metadata.IsSymlink = true
		target, err := os.Readlink(path)
		if err != nil {
			return nil, fmt.Errorf("error reading symlink: %w", err)
		}
		metadata.LinkTarget = target
	}

	// Obtener propietario y grupo
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		metadata.Owner = fmt.Sprint(stat.Uid)
		metadata.Group = fmt.Sprint(stat.Gid)
	}

	// Calcular checksum si el archivo no es un directorio ni un symlink
	if !metadata.IsDir && !metadata.IsSymlink {
		checksum, err := s.calculateChecksum(path)
		if err != nil {
			return nil, fmt.Errorf("error calculating checksum: %w", err)
		}
		metadata.Checksum = checksum
	}

	return metadata, nil
}

func (s *SyncService) shouldSkipFile(path string, options domain.SyncOptions) bool {
	// Verificar patrones de exclusión
	for _, pattern := range options.Exclude {
		matched, err := regexp.MatchString(pattern, path)
		if err == nil && matched {
			return true
		}
	}

	// Verificar patrones de inclusión si existen
	if len(options.Include) > 0 {
		included := false
		for _, pattern := range options.Include {
			matched, err := regexp.MatchString(pattern, path)
			if err == nil && matched {
				included = true
				break
			}
		}
		return !included
	}

	return false
}

func (s *SyncService) needsSync(src, target *domain.FileMetadata, options domain.SyncOptions) bool {
	// Si se requiere checksum, comparar checksums
	if options.Checksum {
		return src.Checksum != target.Checksum
	}

	// Comparar tamaño y tiempo de modificación
	if src.Size != target.Size {
		return true
	}

	// Permitir una pequeña diferencia en los timestamps (1 segundo)
	timeDiff := src.ModTime.Sub(target.ModTime)
	if timeDiff < -time.Second || timeDiff > time.Second {
		return true
	}

	// Si se preservan permisos, comparar modos
	if options.PreservePerms && src.Mode != target.Mode {
		return true
	}

	return false
}

func (s *SyncService) copyFileWithProgress(ctx context.Context, srcPath, destPath string, stats *domain.SyncStats, options domain.SyncOptions) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error opening source file: %w", err)
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("error getting source file info: %w", err)
	}

	// Crear directorio destino si no existe
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("error creating destination file: %w", err)
	}
	defer destFile.Close()

	// Configurar compresión si está habilitada
	var reader io.Reader = srcFile
	if options.Compress {
		gzReader, err := gzip.NewReader(srcFile)
		if err != nil {
			return fmt.Errorf("error creating gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Copiar con progreso
	buffer := make([]byte, s.bufferSize)
	totalBytes := srcInfo.Size()
	copiedBytes := int64(0)
	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			n, err := reader.Read(buffer)
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("error reading source file: %w", err)
			}

			_, err = destFile.Write(buffer[:n])
			if err != nil {
				return fmt.Errorf("error writing to destination file: %w", err)
			}

			copiedBytes += int64(n)
			progress := float64(copiedBytes) / float64(totalBytes) * 100

			// Actualizar progreso
			s.sendProgress(domain.SyncProgress{
				FilePath:         destPath,
				BytesTransferred: copiedBytes,
				TotalBytes:       totalBytes,
				ProgressPercent:  progress,
				Status: domain.SyncStatus{
					Action:  domain.Copying,
					Details: fmt.Sprintf("Copying %s (%.1f%%)", filepath.Base(srcPath), progress),
				},
			})
		}
	}

	// Actualizar estadísticas
	if stats != nil {
		stats.FilesTransferred++
		stats.TotalSize += totalBytes
		elapsedTime := time.Since(startTime).Seconds()
		if elapsedTime > 0 {
			stats.TransferSpeed = float64(totalBytes) / elapsedTime
		}
	}

	// Preservar metadatos si se solicita
	if err := s.preserveMetadata(srcPath, destPath, srcInfo, options); err != nil {
		return fmt.Errorf("error preserving metadata: %w", err)
	}

	return nil
}

func (s *SyncService) createSymlink(target, path string) error {
	// Eliminar el enlace existente si existe
	if _, err := os.Lstat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("error removing existing symlink: %w", err)
		}
	}

	// Crear el directorio padre si no existe
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("error creating parent directory: %w", err)
	}

	// Crear el nuevo enlace simbólico
	if err := os.Symlink(target, path); err != nil {
		return fmt.Errorf("error creating symlink: %w", err)
	}

	return nil
}

func (s *SyncService) processFile(ctx context.Context, filePath string, request domain.FileSyncRequest, stats *domain.SyncStats) error {
	// Obtener ruta relativa
	relPath, err := filepath.Rel(request.SourcePath, filePath)
	if err != nil {
		return fmt.Errorf("error getting relative path: %w", err)
	}

	// Construir ruta destino
	destPath := filepath.Join(request.DestinationPath, relPath)

	// Obtener información del archivo fuente
	srcInfo, err := os.Lstat(filePath)
	if err != nil {
		return fmt.Errorf("error getting source file info: %w", err)
	}

	// Manejar enlaces simbólicos
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(filePath)
		if err != nil {
			return fmt.Errorf("error reading symlink: %w", err)
		}
		return s.createSymlink(target, destPath)
	}

	// Verificar si el archivo destino existe
	destInfo, err := os.Lstat(destPath)
	needsSync := true

	if err == nil {
		// Si el archivo existe, verificar si necesita sincronización
		srcMeta, err := s.getFileMetadata(filePath, srcInfo)
		if err != nil {
			return err
		}

		destMeta, err := s.getFileMetadata(destPath, destInfo)
		if err != nil {
			return err
		}

		needsSync = s.needsSync(srcMeta, destMeta, request.Options)
	}

	if !needsSync {
		stats.FilesSkipped++
		s.sendProgress(domain.SyncProgress{
			FilePath: relPath,
			Status: domain.SyncStatus{
				Action:  domain.Skipping,
				Details: "File is up to date",
			},
		})
		return nil
	}

	// Copiar el archivo con progreso
	return s.copyFileWithProgress(ctx, filePath, destPath, stats, request.Options)
}

func (s *SyncService) calculateChecksumFromBuffer(buffer []byte) string {
	hash := sha256.Sum256(buffer)
	return fmt.Sprintf("%x", hash[:])
}

func (s *SyncService) deleteFile(path string, stats *domain.SyncStats) error {
	err := os.Remove(path)
	if err != nil {
		return fmt.Errorf("error deleting file: %w", err)
	}
	stats.FilesDeleted++
	return nil
}
