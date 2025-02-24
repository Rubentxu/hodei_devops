package service

import (
	"io"

	"dev.rubentxu.devops-platform/archiva-go/internal/domain/artifact"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/repository"
)

type ArtifactService interface {
	StoreArtifact(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact, content io.Reader) error
	RetrieveArtifact(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (io.ReadCloser, error)
}

type artifactService struct {
	storageBackend repository.StorageBackend
}

func NewArtifactService(storageBackend repository.StorageBackend) ArtifactService {
	return &artifactService{storageBackend: storageBackend}
}

func (s *artifactService) StoreArtifact(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact, content io.Reader) error {
	return s.storageBackend.SaveArtifact(mavenArtifact, content)
}

func (s *artifactService) RetrieveArtifact(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (io.ReadCloser, error) {
	return s.storageBackend.GetArtifactContent(mavenArtifact)
}
