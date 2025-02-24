package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"dev.rubentxu.devops-platform/archiva-go/internal/domain/artifact"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/repository"
)

type FileSystemStorage struct {
	rootDir string
}

func NewFileSystemStorage(rootDir string) repository.StorageBackend {
	return &FileSystemStorage{rootDir: rootDir}
}

func (fs *FileSystemStorage) SaveArtifact(mavenArtifact *artifact.MavenArtifact, content io.Reader) error {
	artifactPath := filepath.Join(fs.rootDir, mavenArtifact.RepoId, mavenArtifact.GroupID, mavenArtifact.ArtifactID, mavenArtifact.Version, mavenArtifact.Filename)
	dirPath := filepath.Dir(artifactPath)

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("error creating directories: %w", err)
	}

	outFile, err := os.Create(artifactPath)
	if err != nil {
		return fmt.Errorf("error creating artifact file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, content)
	if err != nil {
		return fmt.Errorf("error writing artifact content: %w", err)
	}
	return nil
}

func (fs *FileSystemStorage) GetArtifactContent(mavenArtifact *artifact.MavenArtifact) (io.ReadCloser, error) {
	artifactPath := filepath.Join(fs.rootDir, mavenArtifact.RepoId, mavenArtifact.GroupID, mavenArtifact.ArtifactID, mavenArtifact.Version, mavenArtifact.Filename)
	file, err := os.Open(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("error opening artifact file: %w", err)
	}
	return file, nil
}

func (fs *FileSystemStorage) GetArtifactMetadata(mavenArtifact *artifact.MavenArtifact) (*artifact.MavenMetadata, error) {
	return nil, nil
}

func (fs *FileSystemStorage) SaveArtifactMetadata(metadata *artifact.MavenMetadata) error {
	return nil
}

func (fs *FileSystemStorage) ArtifactExists(mavenArtifact *artifact.MavenArtifact) bool {
	artifactPath := filepath.Join(fs.rootDir, mavenArtifact.RepoId, mavenArtifact.GroupID, mavenArtifact.ArtifactID, mavenArtifact.Version, mavenArtifact.Filename)
	_, err := os.Stat(artifactPath)
	return !os.IsNotExist(err)
}
