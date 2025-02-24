package repository

import (
	"io"

	"dev.rubentxu.devops-platform/archiva-go/internal/domain/artifact"
)

type Repository struct {
	ID             string
	Name           string
	Type           RepoType
	Format         RepoFormat
	StorageBackend StorageBackend
	Config         RepoConfig
}

type RepoType string

const (
	HOSTED RepoType = "HOSTED"
	PROXY  RepoType = "PROXY"
	GROUP  RepoType = "GROUP"
)

type RepoFormat string

const (
	MAVEN RepoFormat = "MAVEN"
	NPM   RepoFormat = "NPM"
)

type RepoConfig struct {
	ProxyConfig *ProxyRepoConfig `json:"proxy_config,omitempty"`
}

type ProxyRepoConfig struct {
	UpstreamURL string `json:"upstream_url"`
}

type StorageBackend interface {
	SaveArtifact(artifact *artifact.MavenArtifact, content io.Reader) error
	GetArtifactContent(artifact *artifact.MavenArtifact) (io.ReadCloser, error)
	GetArtifactMetadata(artifact *artifact.MavenArtifact) (*artifact.MavenMetadata, error)
	SaveArtifactMetadata(metadata *artifact.MavenMetadata) error
	ArtifactExists(artifact *artifact.MavenArtifact) bool
}
