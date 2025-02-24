package http

import (
	"fmt"
	"io"
	"net/http"

	"dev.rubentxu.devops-platform/archiva-go/internal/app/artifact_app"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/artifact"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/repository"
	"github.com/gorilla/mux"
)

type ArtifactHandlers struct {
	artifactApp *artifact_app.ArtifactApp
}

func NewArtifactHandlers(artifactApp *artifact_app.ArtifactApp) *ArtifactHandlers {
	return &ArtifactHandlers{artifactApp: artifactApp}
}

func (h *ArtifactHandlers) UploadArtifactHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repoId := vars["repoId"]
	groupId := vars["groupId"]
	artifactId := vars["artifactId"]
	version := vars["version"]
	filename := vars["filename"]

	mavenArtifact := &artifact.MavenArtifact{
		RepoId:     repoId,
		GroupID:    groupId,
		ArtifactID: artifactId,
		Version:    version,
		Filename:   filename,
		Path:       "",
	}

	repo := &repository.Repository{ID: repoId, Type: repository.HOSTED, Format: repository.MAVEN}

	err := h.artifactApp.UploadArtifactUseCase(repo, mavenArtifact, r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al subir artefacto: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Artefacto subido correctamente a repositorio '%s': %s\n", repoId, mavenArtifact.Path)
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintln(w, "Artefacto subido correctamente")
}

func (h *ArtifactHandlers) DownloadArtifactHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	repoId := vars["repoId"]
	groupId := vars["groupId"]
	artifactId := vars["artifactId"]
	version := vars["version"]
	filename := vars["filename"]

	mavenArtifact := &artifact.MavenArtifact{
		RepoId:     repoId,
		GroupID:    groupId,
		ArtifactID: artifactId,
		Version:    version,
		Filename:   filename,
		Path:       "",
	}

	repo := &repository.Repository{ID: repoId, Type: repository.HOSTED, Format: repository.MAVEN}

	artifactContent, err := h.artifactApp.DownloadArtifactUseCase(repo, mavenArtifact)
	if err != nil {
		http.Error(w, fmt.Sprintf("Artefacto no encontrado o error al descargar: %v", err), http.StatusNotFound)
		return
	}
	defer artifactContent.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("Content-Type", "application/octet-stream")

	_, err = io.Copy(w, artifactContent)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error al servir el artefacto: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Printf("Artefacto descargado correctamente desde repositorio '%s': %s\n", repoId, mavenArtifact.Path)
}
