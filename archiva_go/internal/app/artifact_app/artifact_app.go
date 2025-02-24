package artifact_app

import (
	"io"

	"dev.rubentxu.devops-platform/archiva-go/internal/domain/artifact"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/repository"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/service"
)

type ArtifactApp struct {
	artifactService service.ArtifactService
	metadataService service.MetadataService
}

func NewArtifactApp(artifactService service.ArtifactService, metadataService service.MetadataService) *ArtifactApp {
	return &ArtifactApp{artifactService: artifactService, metadataService: metadataService}
}

func (app *ArtifactApp) UploadArtifactUseCase(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact, content io.Reader) error {
	err := app.artifactService.StoreArtifact(repo, mavenArtifact, content)
	if err != nil {
		return err
	}
	return nil
}

func (app *ArtifactApp) DownloadArtifactUseCase(repo *repository.Repository, mavenArtifact *artifact.MavenArtifact) (io.ReadCloser, error) {
	return app.artifactService.RetrieveArtifact(repo, mavenArtifact)
}
