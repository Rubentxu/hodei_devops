package domain

import (
	"os"
	"time"
)

// FileMetadata contiene todos los metadatos de un archivo
type FileMetadata struct {
	Path       string
	Size       int64
	ModTime    time.Time
	Mode       os.FileMode
	IsDir      bool
	IsSymlink  bool
	Owner      string
	Group      string
	Checksum   string
	ACLs       []ACLEntry
	XAttrs     map[string][]byte
	LinkTarget string // Para symlinks
}

type ACLEntry struct {
	Principal string
	Perms     string
	Type      string // user, group, other
}

// DeltaBlock representa un bloque de datos para transferencia delta
type DeltaBlock struct {
	Offset    int64
	Size      int32
	Checksum  string
	Data      []byte
	MatchRef  int64 // Referencia al bloque existente en el archivo destino
	IsLiteral bool  // true si es un bloque nuevo, false si es una referencia
}

// FileChange representa un cambio detectado en un archivo
type FileChange struct {
	Path           string
	ChangeType     ChangeType
	SourceMetadata *FileMetadata
	TargetMetadata *FileMetadata
}

type ChangeType int

const (
	Create ChangeType = iota
	Modify
	Delete
	NoChange
)

// ListFilesRequest contiene los parámetros para listar archivos
type ListFilesRequest struct {
	Path      string   // Ruta a listar
	Recursive bool     // Si debe listar recursivamente
	Include   []string // Patrones de inclusión
	Exclude   []string // Patrones de exclusión
}

// DeleteFileRequest contiene los parámetros para eliminar archivos
type DeleteFileRequest struct {
	Path      string // Ruta del archivo a eliminar
	Recursive bool   // Si debe eliminar recursivamente
	Force     bool   // Si debe forzar la eliminación
}

// FileUpload representa una transferencia de archivo en curso
type FileUpload struct {
	Metadata *FileSyncMetadata
	File     *os.File
	Progress *SyncProgress
}

// FileSyncMetadata contiene la información inicial de un archivo a sincronizar
type FileSyncMetadata struct {
	SourcePath      string
	DestinationPath string
	TotalSize       int64
	Options         SyncOptions
}

// FileChunk representa un fragmento de archivo durante la transferencia
type FileChunk struct {
	Content  []byte
	Offset   int64
	IsLast   bool
	DeltaRef *DeltaReference
}

// DeltaReference referencia a un bloque existente en el archivo destino
type DeltaReference struct {
	SourceOffset int64
	Length       int32
	Checksum     string
}

// FileSyncRequest contiene los parámetros para una solicitud de sincronización
type FileSyncRequest struct {
	SourcePath      string
	DestinationPath string
	Options         SyncOptions
}

// SyncOptions define las opciones de sincronización
type SyncOptions struct {
	Recursive      bool
	Archive        bool
	Compress       bool
	Delete         bool
	DryRun         bool
	PreservePerms  bool
	PreserveTimes  bool
	PreserveOwner  bool
	PreserveGroup  bool
	Checksum       bool
	UseDelta       bool  // Usar transferencia delta
	BlockSize      int32 // Tamaño de bloque para delta sync
	CompressLevel  int32
	BandwidthLimit string
	MaxDepth       int32
	Include        []string
	Exclude        []string
}

type SyncProgress struct {
	FilePath         string
	BytesTransferred int64
	TotalBytes       int64
	Status           SyncStatus
	ErrorMessage     string
	ProgressPercent  float64
	Stats            SyncStats
}

type SyncStatus struct {
	Action  SyncAction
	Details string
}

type SyncAction int

const (
	Unknown SyncAction = iota
	Scanning
	Comparing
	Copying
	Deleting
	Linking
	Skipping
	Verifying
	Done
	Error
)

type SyncStats struct {
	FilesTransferred int64
	FilesDeleted     int64
	FilesSkipped     int64
	TotalFiles       int64
	TotalSize        int64
	TransferSpeed    float64
	Errors           int64
	ErrorList        []string
}
