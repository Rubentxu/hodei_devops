package service

import (
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/artifact"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/repository"
)

type MetadataService interface {
	GenerateMetadata(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (*artifact.MavenMetadata, error)
	SaveMetadata(repo *repository.Repository, metadata *artifact.MavenMetadata) error
	GetMetadata(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (*artifact.MavenMetadata, error)
}

type metadataService struct {
	storageBackend repository.StorageBackend
}

func NewMetadataService(storageBackend repository.StorageBackend) MetadataService {
	return &metadataService{storageBackend: storageBackend}
}

func (s *metadataService) GenerateMetadata(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (*artifact.MavenMetadata, error) {
	return nil, nil
}

func (s *metadataService) SaveMetadata(repo *repository.Repository, metadata *artifact.MavenMetadata) error {
	return s.storageBackend.SaveArtifactMetadata(metadata)
}

func (s *metadataService) GetMetadata(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (*artifact.MavenMetadata, error) {
	return s.storageBackend.GetArtifactMetadata(mavenArtifact)
}
