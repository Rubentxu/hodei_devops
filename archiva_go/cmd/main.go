package main

import (
	"fmt"
	net "net/http"

	"dev.rubentxu.devops-platform/archiva-go/config"
	"dev.rubentxu.devops-platform/archiva-go/internal/app/artifact_app"
	"dev.rubentxu.devops-platform/archiva-go/internal/domain/service"
	"dev.rubentxu.devops-platform/archiva-go/internal/infrastructure/storage"
	"dev.rubentxu.devops-platform/archiva-go/internal/interfaces/http"
	"github.com/gorilla/mux"
)

func main() {
	cfg := config.LoadConfig()

	fileStorage := storage.NewFileSystemStorage(cfg.RepoDir)

	artifactService := service.NewArtifactService(fileStorage)
	metadataService := service.NewMetadataService(fileStorage)

	artifactApp := artifact_app.NewArtifactApp(artifactService, metadataService)

	artifactHandlers := http.NewArtifactHandlers(artifactApp)

	router := mux.NewRouter()

	router.HandleFunc("/repository/{repoId}/{groupId}/{artifactId}/{version}/{filename}", artifactHandlers.UploadArtifactHandler).Methods("PUT")
	router.HandleFunc("/repository/{repoId}/{groupId}/{artifactId}/{version}/{filename}", artifactHandlers.DownloadArtifactHandler).Methods("GET")

	fmt.Println("Servidor escuchando en el puerto 8080")
	net.ListenAndServe(":8082", router)
}
